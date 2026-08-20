package app

import (
	"log/slog"
	"net/http"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	"github.com/esuEdu/go-tauri-discord/internal/config"
	"github.com/esuEdu/go-tauri-discord/internal/db"
	"github.com/esuEdu/go-tauri-discord/internal/gateway"
	"github.com/esuEdu/go-tauri-discord/internal/guild"
	"github.com/esuEdu/go-tauri-discord/internal/message"
	"github.com/esuEdu/go-tauri-discord/internal/platform/bus"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
	"github.com/esuEdu/go-tauri-discord/internal/platform/pubsub"
	"github.com/esuEdu/go-tauri-discord/internal/platform/ratelimit"
	"github.com/esuEdu/go-tauri-discord/internal/voice"
)

type App struct {
	Handler http.Handler
	Gateway *gateway.Gateway
	limits  *limits
	voice   *voice.SFU
}

func New(cfg config.Config, pool *db.Pool, broker pubsub.Broker) *App {
	publisher := bus.NewPublisher(broker)

	lim := newLimits(cfg)
	trusted := ratelimit.ParsePrefixes(cfg.TrustedProxies)

	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL)
	var loginThrottle auth.Throttle
	if !cfg.RateLimitDisabled {
		loginThrottle = lim.loginAccount
	}
	authSvc := auth.NewService(pool, pool, tokens, cfg.RefreshTokenTTL, loginThrottle)
	guildSvc := guild.NewService(pool, pool, pool, publisher)
	messageSvc := message.NewService(pool, guildSvc, publisher)

	gw := gateway.New(authSvc, guildSvc, messageSvc, broker, cfg.HeartbeatInterval, OriginHosts(cfg.CORSOrigins), cfg.MaxSessionsPerUser)

	application := &App{Gateway: gw, limits: lim}

	mux := http.NewServeMux()
	guard := authSvc.RequireAuth
	if !cfg.RateLimitDisabled {
		authedLimit := lim.authedMiddleware()
		guard = func(next http.Handler) http.Handler {
			return authSvc.RequireAuth(authedLimit(next))
		}
	}
	protected := httpx.Guarded{Mux: mux, MW: guard}

	authHandler := auth.NewHandler(authSvc, gw)
	authHandler.Routes(mux)
	protected.HandleFunc("GET /api/v1/users/@me", authHandler.Me)
	protected.HandleFunc("DELETE /api/v1/users/@me", authHandler.DeleteMe)
	guildHandler := guild.NewHandler(guildSvc, publisher)
	guildHandler.Routes(protected)
	guildHandler.PublicRoutes(mux)
	message.NewHandler(messageSvc).Routes(protected)

	mux.HandleFunc("GET /gateway", gw.Handler())
	mux.HandleFunc("GET /healthz", healthz(pool, gw))

	if cfg.UIDir != "" {
		mux.HandleFunc("/", spaHandler(cfg.UIDir))
	}

	middleware := []httpx.Middleware{
		httpx.Recover,
		httpx.RequestID,
		httpx.Logger,
		httpx.CORS(cfg.CORSOrigins),
	}
	if !cfg.RateLimitDisabled {
		middleware = append(middleware, lim.publicMiddleware(trusted))
	}
	handler := httpx.Chain(mux, middleware...)

	if !cfg.VoiceDisabled {
		sfu, err := voice.New(gw, cfg.ICEServers)
		if err != nil {
			slog.Error("voice disabled: could not start the SFU", "error", err)
		} else {
			gw.AttachVoice(sfu)
			application.voice = sfu
		}
	}

	application.Handler = handler
	return application
}

func (a *App) Close() {
	a.Gateway.Close()
	a.limits.stop()
	if a.voice != nil {
		a.voice.Close()
	}
}

func healthz(pool *db.Pool, gw *gateway.Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Raw().Ping(r.Context()); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"sessions": gw.SessionCount(),
		})
	}
}

func OriginHosts(origins []string) []string {
	hosts := make([]string, 0, len(origins))
	for _, o := range origins {
		host := o
		for _, scheme := range []string{"http://", "https://", "tauri://", "ws://", "wss://"} {
			if len(host) > len(scheme) && host[:len(scheme)] == scheme {
				host = host[len(scheme):]
				break
			}
		}
		hosts = append(hosts, host)
	}
	return hosts
}
