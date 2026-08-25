package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/esuEdu/go-tauri-discord/internal/ice"
	"github.com/esuEdu/go-tauri-discord/internal/storage"
)

type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CORSOrigins     []string

	UIDir              string
	TrustedProxies     []string
	MaxSessionsPerUser int

	RateLimitDisabled bool
	RegisterPerHour   int
	LoginPerMinute    int
	MessagesPerMinute int

	StorageKind string
	StorageDir  string
	S3          storage.S3Config

	ICEServers    []ice.Server
	TURNSecret    string
	TURNTTL       time.Duration
	VoiceDisabled bool

	HeartbeatInterval time.Duration
}

func (c Config) IsProduction() bool { return c.Env == "production" }

const devJWTSecret = "dev-only-insecure-secret-change-me-0123456789"

func Load() (Config, error) {
	c := Config{
		Env:                env("ENV", "development"),
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		DatabaseURL:        env("DATABASE_URL", "postgres://vocalis:vocalis@localhost:5432/vocalis?sslmode=disable"),
		AccessTokenTTL:     envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:    envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		HeartbeatInterval:  envDuration("HEARTBEAT_INTERVAL", 30*time.Second),
		UIDir:              env("UI_DIR", ""),
		MaxSessionsPerUser: envInt("MAX_SESSIONS_PER_USER", 5),
		TrustedProxies:     strings.Split(env("TRUSTED_PROXIES", "127.0.0.1/32,::1/128"), ","),
		RateLimitDisabled:  env("RATE_LIMIT_DISABLED", "") == "true",
		RegisterPerHour:    envInt("REGISTER_PER_HOUR", 10),
		LoginPerMinute:     envInt("LOGIN_PER_MINUTE", 20),
		MessagesPerMinute:  envInt("MESSAGES_PER_MINUTE", 60),
		StorageKind:        env("STORAGE", "disk"),
		StorageDir:         env("STORAGE_DIR", "./data/files"),
		S3: storage.S3Config{
			Endpoint:  env("S3_ENDPOINT", "localhost:9000"),
			Bucket:    env("S3_BUCKET", "vocalis"),
			AccessKey: env("S3_ACCESS_KEY", ""),
			Secret:    env("S3_SECRET_KEY", ""),
			Region:    env("S3_REGION", "us-east-1"),
			UseSSL:    env("S3_USE_SSL", "") == "true",
		},
		TURNSecret:    env("TURN_SECRET", ""),
		TURNTTL:       envDuration("TURN_TTL", 12*time.Hour),
		VoiceDisabled: env("VOICE_DISABLED", "") == "true",
		CORSOrigins: strings.Split(
			env("CORS_ORIGINS", "http://localhost:1420,tauri://localhost"), ","),
	}

	servers, err := ice.ParseServers(env("ICE_SERVERS", "stun:stun.l.google.com:19302"))
	if err != nil {
		return Config{}, fmt.Errorf("ICE_SERVERS: %w", err)
	}
	c.ICEServers = servers

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

func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
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
