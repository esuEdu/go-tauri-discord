package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

type TokenIssuer struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenIssuer(secret []byte, accessTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: secret, accessTTL: accessTTL}
}

const issuer = "go-tauri-discord"

func (t *TokenIssuer) IssueAccess(userID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(t.accessTTL)
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
		ID:        uuid.NewString(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, domain.Internal(err)
	}
	return signed, exp, nil
}

func (t *TokenIssuer) ParseAccess(raw string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) {
		return t.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return uuid.Nil, domain.Unauthorized("invalid or expired token")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, domain.Unauthorized("invalid token subject")
	}
	return userID, nil
}

const refreshTokenBytes = 32

func newRefreshToken() (plain string, hash []byte, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, domain.Internal(err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, hashRefreshToken(plain), nil
}

func hashRefreshToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}
