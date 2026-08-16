package app

import (
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
)

type App struct {
	Handler http.Handler
	Gateway *gateway.Gateway
}

func New(cfg config.Config, pool *db.Pool, broker pubsub.Broker) *App {
	publisher := bus.NewPublisher(broker)

	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL)
	authSvc := auth.NewService(pool, tokens, cfg.RefreshTokenTTL)
	guildSvc := guild.NewService(pool, pool)
	messageSvc := message.NewService(pool, guildSvc, publisher)

	gw := gateway.New(authSvc, guildSvc, broker, cfg.HeartbeatInterval, OriginHosts(cfg.CORSOrigins))

	mux := http.NewServeMux()
	protected := httpx.Guarded{Mux: mux, MW: authSvc.RequireAuth}

	authHandler := auth.NewHandler(authSvc)
	authHandler.Routes(mux)
	protected.HandleFunc("GET /api/v1/users/@me", authHandler.Me)
	guild.NewHandler(guildSvc, publisher).Routes(protected)
	message.NewHandler(messageSvc).Routes(protected)

	mux.HandleFunc("GET /gateway", gw.Handler())
	mux.HandleFunc("GET /healthz", healthz(pool, gw))

	handler := httpx.Chain(mux,
		httpx.Recover,
		httpx.RequestID,
		httpx.Logger,
		httpx.CORS(cfg.CORSOrigins),
	)

	return &App{Handler: handler, Gateway: gw}
}

func (a *App) Close() { a.Gateway.Close() }

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
