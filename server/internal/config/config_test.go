package config

import (
	"slices"
	"testing"
)

// Tauri does not present one origin. macOS and Linux send tauri://localhost;
// Windows runs on WebView2 and sends http://tauri.localhost. A list carrying
// only the first is correct on the machine any of this gets written on and
// breaks registration on Windows, where the preflight is answered without a
// matching allow-origin, the webview never sends the real request, and the app
// reports that the server did not answer.
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
