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

	"github.com/esuEdu/go-tauri-discord/internal/app"
	"github.com/esuEdu/go-tauri-discord/internal/config"
	"github.com/esuEdu/go-tauri-discord/internal/db"
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

	if cfg.MigrateOnStart {
		if err := db.Migrate(ctx, pool); err != nil {
			return err
		}
	}

	broker := pubsub.NewMemory()
	defer broker.Close()

	application := app.New(cfg, pool, broker)
	defer application.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           application.Handler,
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

	application.Close()

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
