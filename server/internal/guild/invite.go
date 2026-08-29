package guild

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

type InviteRepository interface {
	CreateInvite(ctx context.Context, arg dbgen.CreateInviteParams) (dbgen.Invite, error)
	GetInvite(ctx context.Context, code string) (dbgen.Invite, error)
	ListGuildInvites(ctx context.Context, guildID uuid.UUID) ([]dbgen.Invite, error)
	RevokeInvite(ctx context.Context, code string) error
	CountGuildMembers(ctx context.Context, guildID uuid.UUID) (int64, error)
}

const (
	inviteCodeLength  = 8
	inviteAlphabet    = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	maxInviteAttempts = 5
	MaxInviteAge      = 30 * 24 * time.Hour
)

type InviteView struct {
	Code        string     `json:"code"`
	GuildID     uuid.UUID  `json:"guild_id"`
	GuildName   string     `json:"guild_name"`
	InviterID   uuid.UUID  `json:"inviter_id"`
	MemberCount int64      `json:"member_count"`
	Uses        int32      `json:"uses"`
	MaxUses     *int32     `json:"max_uses"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func newInviteCode() (string, error) {
	buf := make([]byte, inviteCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", domain.Internal(err)
	}
	out := make([]byte, inviteCodeLength)
	for i, b := range buf {
		out[i] = inviteAlphabet[int(b)%len(inviteAlphabet)]
	}
	return string(out), nil
}

func (s *Service) CreateInvite(ctx context.Context, userID, guildID uuid.UUID, maxUses *int32, expiresIn *time.Duration) (InviteView, error) {
	perms, err := s.PermissionsInGuild(ctx, userID, guildID)
	if err != nil {
		return InviteView{}, err
	}
	if !perms.Has(domain.PermCreateInvite) {
		return InviteView{}, domain.Forbidden("missing CreateInvite permission")
	}
	if maxUses != nil && *maxUses < 1 {
		return InviteView{}, domain.Invalid("max_uses must be at least 1")
	}

	age := MaxInviteAge
	if expiresIn != nil {
		if *expiresIn <= 0 {
			return InviteView{}, domain.Invalid("expires_in must be positive")
		}
		age = min(*expiresIn, MaxInviteAge)
	}
	deadline := time.Now().Add(age)

	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		return InviteView{}, domain.Internal(err)
	}

	for attempt := range maxInviteAttempts {
		code, err := newInviteCode()
		if err != nil {
			return InviteView{}, err
		}

		invite, err := s.invites.CreateInvite(ctx, dbgen.CreateInviteParams{
			Code:      code,
			GuildID:   guildID,
			InviterID: userID,
			MaxUses:   maxUses,
			ExpiresAt: &deadline,
		})
		if err != nil {
			if db.IsUniqueViolation(err) && attempt < maxInviteAttempts-1 {
				continue
			}
			return InviteView{}, domain.Internal(err)
		}

		count, err := s.invites.CountGuildMembers(ctx, guildID)
		if err != nil {
			return InviteView{}, domain.Internal(err)
		}
		return toInviteView(invite, guild.Name, count), nil
	}
	return InviteView{}, domain.Conflict("could not allocate an invite code")
}

func (s *Service) PreviewInvite(ctx context.Context, code string) (InviteView, error) {
	invite, err := s.invites.GetInvite(ctx, code)
	if err != nil {
		if db.IsNoRows(err) {
			return InviteView{}, domain.NotFound("invite")
		}
		return InviteView{}, domain.Internal(err)
	}
	if inviteUnusable(invite) != "" {
		return InviteView{}, domain.NotFound("invite")
	}

	guild, err := s.repo.GetGuild(ctx, invite.GuildID)
	if err != nil {
		return InviteView{}, domain.Internal(err)
	}
	count, err := s.invites.CountGuildMembers(ctx, invite.GuildID)
	if err != nil {
		return InviteView{}, domain.Internal(err)
	}
	return toInviteView(invite, guild.Name, count), nil
}

func (s *Service) RedeemInvite(ctx context.Context, userID uuid.UUID, code string) (dbgen.Guild, error) {
	invite, err := s.invites.GetInvite(ctx, code)
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.Guild{}, domain.NotFound("invite")
		}
		return dbgen.Guild{}, domain.Internal(err)
	}

	guild, err := s.repo.GetGuild(ctx, invite.GuildID)
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.Guild{}, domain.NotFound("guild")
		}
		return dbgen.Guild{}, domain.Internal(err)
	}

	banned, err := s.Banned(ctx, invite.GuildID, userID)
	if err != nil {
		return dbgen.Guild{}, err
	}
	if banned {
		return dbgen.Guild{}, domain.Forbidden("you are banned from this server")
	}

	if _, err := s.repo.GetGuildMember(ctx, dbgen.GetGuildMemberParams{
		GuildID: invite.GuildID, UserID: userID,
	}); err == nil {
		return guild, nil
	} else if !db.IsNoRows(err) {
		return dbgen.Guild{}, domain.Internal(err)
	}

	err = s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.ConsumeInvite(ctx, code); err != nil {
			if db.IsNoRows(err) {
				return domain.Forbidden("invite is expired, revoked or fully used")
			}
			return domain.Internal(err)
		}
		_, err := q.AddGuildMember(ctx, dbgen.AddGuildMemberParams{
			GuildID: invite.GuildID, UserID: userID,
		})
		return err
	})
	if err != nil {
		if domain.KindOf(err) != domain.KindInternal {
			return dbgen.Guild{}, err
		}
		return dbgen.Guild{}, domain.Internal(err)
	}
	return guild, nil
}

func (s *Service) RevokeInvite(ctx context.Context, userID uuid.UUID, code string) error {
	invite, err := s.invites.GetInvite(ctx, code)
	if err != nil {
		if db.IsNoRows(err) {
			return domain.NotFound("invite")
		}
		return domain.Internal(err)
	}

	perms, err := s.PermissionsInGuild(ctx, userID, invite.GuildID)
	if err != nil {
		return err
	}
	if invite.InviterID != userID && !perms.Has(domain.PermManageGuild) {
		return domain.Forbidden("missing ManageGuild permission")
	}
	if err := s.invites.RevokeInvite(ctx, code); err != nil {
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) ListInvites(ctx context.Context, userID, guildID uuid.UUID) ([]InviteView, error) {
	perms, err := s.PermissionsInGuild(ctx, userID, guildID)
	if err != nil {
		return nil, err
	}
	if !perms.Has(domain.PermManageGuild) {
		return nil, domain.Forbidden("missing ManageGuild permission")
	}

	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	rows, err := s.invites.ListGuildInvites(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	count, err := s.invites.CountGuildMembers(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}

	out := make([]InviteView, 0, len(rows))
	for _, invite := range rows {
		if inviteUnusable(invite) != "" {
			continue
		}
		out = append(out, toInviteView(invite, guild.Name, count))
	}
	return out, nil
}

func inviteUnusable(invite dbgen.Invite) string {
	switch {
	case invite.RevokedAt != nil:
		return "revoked"
	case invite.ExpiresAt != nil && invite.ExpiresAt.Before(time.Now()):
		return "expired"
	case invite.MaxUses != nil && invite.Uses >= *invite.MaxUses:
		return "exhausted"
	default:
		return ""
	}
}

func toInviteView(invite dbgen.Invite, guildName string, memberCount int64) InviteView {
	return InviteView{
		Code:        invite.Code,
		GuildID:     invite.GuildID,
		GuildName:   guildName,
		InviterID:   invite.InviterID,
		MemberCount: memberCount,
		Uses:        invite.Uses,
		MaxUses:     invite.MaxUses,
		ExpiresAt:   invite.ExpiresAt,
		CreatedAt:   invite.CreatedAt,
	}
}
