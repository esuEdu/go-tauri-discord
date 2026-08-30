package config

import (
	"slices"
	"testing"
)

func TestDefaultCORSOriginsCoverEveryDesktopPlatform(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:5432/d?sslmode=disable")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, origin := range []string{"tauri://localhost", "http://tauri.localhost"} {
		if !slices.Contains(cfg.CORSOrigins, origin) {
			t.Errorf("default CORS origins %q do not allow %s", cfg.CORSOrigins, origin)
		}
	}
}
