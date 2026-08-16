package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CORSOrigins     []string

	HeartbeatInterval time.Duration
}

func (c Config) IsProduction() bool { return c.Env == "production" }

const devJWTSecret = "dev-only-insecure-secret-change-me-0123456789"

func Load() (Config, error) {
	c := Config{
		Env:               env("ENV", "development"),
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		DatabaseURL:       env("DATABASE_URL", "postgres://vocalis:vocalis@localhost:5432/vocalis?sslmode=disable"),
		AccessTokenTTL:    envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:   envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		HeartbeatInterval: envDuration("HEARTBEAT_INTERVAL", 30*time.Second),
		CORSOrigins: strings.Split(
			env("CORS_ORIGINS", "http://localhost:1420,tauri://localhost"), ","),
	}

	secret := env("JWT_SECRET", "")
	if secret == "" {
		if c.IsProduction() {
			return Config{}, fmt.Errorf("JWT_SECRET is required when ENV=production")
		}
		secret = devJWTSecret
	}
	if len(secret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes, got %d", len(secret))
	}
	c.JWTSecret = []byte(secret)

	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}

	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return fallback
}
