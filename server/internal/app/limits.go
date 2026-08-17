package app

import (
	"net/http"
	"net/netip"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	"github.com/esuEdu/go-tauri-discord/internal/config"
	"github.com/esuEdu/go-tauri-discord/internal/platform/ratelimit"
)

type limits struct {
	register      *ratelimit.Limiter
	login         *ratelimit.Limiter
	loginAccount  *ratelimit.Limiter
	refresh       *ratelimit.Limiter
	invitePreview *ratelimit.Limiter
	publicOther   *ratelimit.Limiter

	messages    *ratelimit.Limiter
	typing      *ratelimit.Limiter
	inviteMint  *ratelimit.Limiter
	guildCreate *ratelimit.Limiter
	authedOther *ratelimit.Limiter
}

func newLimits(cfg config.Config) *limits {
	return &limits{
		register:      ratelimit.New(ratelimit.PerHour(cfg.RegisterPerHour, 3)),
		login:         ratelimit.New(ratelimit.PerMinute(cfg.LoginPerMinute, 10)),
		loginAccount:  ratelimit.New(ratelimit.PerMinute(5, 5)),
		refresh:       ratelimit.New(ratelimit.PerMinute(30, 10)),
		invitePreview: ratelimit.New(ratelimit.PerMinute(60, 20)),
		publicOther:   ratelimit.New(ratelimit.PerMinute(120, 60)),

		messages:    ratelimit.New(ratelimit.PerMinute(cfg.MessagesPerMinute, 10)),
		typing:      ratelimit.New(ratelimit.PerMinute(12, 3)),
		inviteMint:  ratelimit.New(ratelimit.PerMinute(10, 5)),
		guildCreate: ratelimit.New(ratelimit.PerHour(20, 5)),
		authedOther: ratelimit.New(ratelimit.PerMinute(300, 100)),
	}
}

func (l *limits) stop() {
	for _, limiter := range []*ratelimit.Limiter{
		l.register, l.login, l.loginAccount, l.refresh, l.invitePreview,
		l.publicOther, l.messages, l.typing, l.inviteMint, l.guildCreate,
		l.authedOther,
	} {
		limiter.Stop()
	}
}

func (l *limits) publicMiddleware(trusted []netip.Prefix) func(http.Handler) http.Handler {
	key := func(r *http.Request) string {
		return ratelimit.ClientIP(r, trusted)
	}
	return ratelimit.Middleware(key, []ratelimit.Rule{
		{Match: ratelimit.Method(http.MethodPost, "/api/v1/auth/register"), Limiter: l.register},
		{Match: ratelimit.Method(http.MethodPost, "/api/v1/auth/login"), Limiter: l.login},
		{Match: ratelimit.Method(http.MethodPost, "/api/v1/auth/refresh"), Limiter: l.refresh},
		{Match: ratelimit.Method(http.MethodGet, "/api/v1/invites/"), Limiter: l.invitePreview},
		{Match: ratelimit.Any("/api/"), Limiter: l.publicOther},
	})
}

func (l *limits) authedMiddleware() func(http.Handler) http.Handler {
	key := func(r *http.Request) string {
		user, ok := auth.UserFrom(r.Context())
		if !ok {
			return ""
		}
		return user.ID.String()
	}
	return ratelimit.Middleware(key, []ratelimit.Rule{
		{Match: ratelimit.MethodSuffix(http.MethodPost, "/api/v1/channels/", "/typing"), Limiter: l.typing},
		{Match: ratelimit.MethodSuffix(http.MethodPost, "/api/v1/channels/", "/messages"), Limiter: l.messages},
		{Match: ratelimit.MethodSuffix(http.MethodPost, "/api/v1/guilds/", "/invites"), Limiter: l.inviteMint},
		{Match: ratelimit.Method(http.MethodPost, "/api/v1/guilds"), Limiter: l.guildCreate},
		{Match: ratelimit.Any("/api/"), Limiter: l.authedOther},
	})
}
