package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
)

type ctxKey int

const userKey ctxKey = iota

func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok {
			httpx.Error(w, r, domain.Unauthorized("missing bearer token"))
			return
		}
		user, err := s.Authenticate(r.Context(), raw)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	raw, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return "", false
	}
	raw = strings.TrimSpace(raw)
	return raw, raw != ""
}

func WithUser(ctx context.Context, user dbgen.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func UserFrom(ctx context.Context) (dbgen.User, bool) {
	user, ok := ctx.Value(userKey).(dbgen.User)
	return user, ok
}

func MustUserID(ctx context.Context) uuid.UUID {
	user, ok := UserFrom(ctx)
	if !ok {
		panic("auth: handler used outside RequireAuth")
	}
	return user.ID
}
