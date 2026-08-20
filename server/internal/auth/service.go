package auth

import (
	"context"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

type Repository interface {
	CreateUser(ctx context.Context, arg dbgen.CreateUserParams) (dbgen.User, error)
	GetUserByEmail(ctx context.Context, email string) (dbgen.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	CreateRefreshToken(ctx context.Context, arg dbgen.CreateRefreshTokenParams) (dbgen.RefreshToken, error)
	GetActiveRefreshToken(ctx context.Context, tokenHash []byte) (dbgen.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID) error
}

type Throttle interface {
	Allow(key string) (bool, time.Duration)
}

type TxRunner interface {
	InTx(ctx context.Context, fn func(q *dbgen.Queries) error) error
}

type Service struct {
	repo       Repository
	tx         TxRunner
	tokens     *TokenIssuer
	refreshTTL time.Duration
	logins     Throttle
}

func NewService(repo Repository, tx TxRunner, tokens *TokenIssuer, refreshTTL time.Duration, logins Throttle) *Service {
	return &Service{repo: repo, tx: tx, tokens: tokens, refreshTTL: refreshTTL, logins: logins}
}

var DeletedUserID = uuid.UUID{}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

const (
	minPasswordLen = 8
	maxPasswordLen = 72
	minUsernameLen = 2
	maxUsernameLen = 32
)

func (s *Service) Register(ctx context.Context, username, email, password string) (dbgen.User, TokenPair, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)

	if n := utf8.RuneCountInString(username); n < minUsernameLen || n > maxUsernameLen {
		return dbgen.User{}, TokenPair{}, domain.Invalid("username must be %d-%d characters", minUsernameLen, maxUsernameLen)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return dbgen.User{}, TokenPair{}, domain.Invalid("invalid email address")
	}
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return dbgen.User{}, TokenPair{}, domain.Invalid("password must be %d-%d bytes", minPasswordLen, maxPasswordLen)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return dbgen.User{}, TokenPair{}, domain.Internal(err)
	}

	user, err := s.repo.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           uuid.Must(uuid.NewV7()),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return dbgen.User{}, TokenPair{}, domain.Conflict("username or email already taken")
		}
		return dbgen.User{}, TokenPair{}, domain.Internal(err)
	}

	pair, err := s.issuePair(ctx, user.ID)
	if err != nil {
		return dbgen.User{}, TokenPair{}, err
	}
	return user, pair, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (dbgen.User, TokenPair, error) {
	email = strings.TrimSpace(email)

	if s.logins != nil {
		if allowed, _ := s.logins.Allow(strings.ToLower(email)); !allowed {
			return dbgen.User{}, TokenPair{}, domain.RateLimited("too many sign-in attempts for this account")
		}
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if db.IsNoRows(err) {

			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return dbgen.User{}, TokenPair{}, domain.Unauthorized("invalid credentials")
		}
		return dbgen.User{}, TokenPair{}, domain.Internal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return dbgen.User{}, TokenPair{}, domain.Unauthorized("invalid credentials")
	}

	pair, err := s.issuePair(ctx, user.ID)
	if err != nil {
		return dbgen.User{}, TokenPair{}, err
	}
	return user, pair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	stored, err := s.repo.GetActiveRefreshToken(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if db.IsNoRows(err) {
			return TokenPair{}, domain.Unauthorized("invalid or expired refresh token")
		}
		return TokenPair{}, domain.Internal(err)
	}

	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return TokenPair{}, domain.Internal(err)
	}
	return s.issuePair(ctx, stored.UserID)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	stored, err := s.repo.GetActiveRefreshToken(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if db.IsNoRows(err) {
			return nil
		}
		return domain.Internal(err)
	}
	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) Authenticate(ctx context.Context, accessToken string) (dbgen.User, error) {
	userID, err := s.tokens.ParseAccess(accessToken)
	if err != nil {
		return dbgen.User{}, err
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if db.IsNoRows(err) {

			return dbgen.User{}, domain.Unauthorized("account no longer exists")
		}
		return dbgen.User{}, domain.Internal(err)
	}
	return user, nil
}

func (s *Service) DeleteAccount(ctx context.Context, userID uuid.UUID, password string) error {
	if userID == DeletedUserID {
		return domain.Forbidden("the deleted-user placeholder cannot be deleted")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if db.IsNoRows(err) {
			return domain.Unauthorized("account no longer exists")
		}
		return domain.Internal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.Unauthorized("invalid credentials")
	}

	err = s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		owned, err := q.ListGuildsOwnedBy(ctx, userID)
		if err != nil {
			return err
		}
		for _, g := range owned {
			heir, err := q.NextGuildOwner(ctx, dbgen.NextGuildOwnerParams{
				GuildID: g.ID, LeavingID: userID,
			})
			if err != nil {
				if !db.IsNoRows(err) {
					return err
				}
				if err := q.DeleteGuild(ctx, g.ID); err != nil {
					return err
				}
				continue
			}
			if err := q.TransferGuildOwnership(ctx, dbgen.TransferGuildOwnershipParams{
				ID: g.ID, OwnerID: heir,
			}); err != nil {
				return err
			}
		}

		if err := q.ReassignMessagesToUser(ctx, dbgen.ReassignMessagesToUserParams{
			AuthorID: userID, NewAuthorID: DeletedUserID,
		}); err != nil {
			return err
		}

		return q.DeleteUser(ctx, userID)
	})
	if err != nil {
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) issuePair(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	access, expiresAt, err := s.tokens.IssueAccess(userID)
	if err != nil {
		return TokenPair{}, err
	}

	plain, hash, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	if _, err := s.repo.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}); err != nil {
		return TokenPair{}, domain.Internal(err)
	}

	return TokenPair{AccessToken: access, RefreshToken: plain, ExpiresAt: expiresAt}, nil
}

var dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
