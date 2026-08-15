package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogging(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("database connected")

	broker := pubsub.NewMemory()
	defer broker.Close()
	publisher := bus.NewPublisher(broker)

	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL)
	authSvc := auth.NewService(pool, tokens, cfg.RefreshTokenTTL)
	guildSvc := guild.NewService(pool, pool)
	messageSvc := message.NewService(pool, guildSvc, publisher)

	gw := gateway.New(authSvc, guildSvc, broker, cfg.HeartbeatInterval, originHosts(cfg.CORSOrigins))
	defer gw.Close()

	mux := http.NewServeMux()
	protected := httpx.Guarded{Mux: mux, MW: authSvc.RequireAuth}

	auth.NewHandler(authSvc).Routes(mux)
	protected.HandleFunc("GET /api/v1/users/@me", auth.NewHandler(authSvc).Me)
	guild.NewHandler(guildSvc, publisher).Routes(protected)
	message.NewHandler(messageSvc).Routes(protected)

	mux.HandleFunc("GET /gateway", gw.Handler())

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Raw().Ping(r.Context()); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"sessions": gw.SessionCount(),
		})
	})

	handler := httpx.Chain(mux,
		httpx.Recover,
		httpx.RequestID,
		httpx.Logger,
		httpx.CORS(cfg.CORSOrigins),
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	gw.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}

func setupLogging(cfg config.Config) {
	level := slog.LevelDebug
	var handler slog.Handler
	if cfg.IsProduction() {
		level = slog.LevelInfo
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

func originHosts(origins []string) []string {
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
