package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
)

func spaHandler(dir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		// An unmatched API path must not fall through to the SPA, or a typo
		// in a route returns index.html with a 200 instead of a 404.
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/gateway" {
			httpx.Error(w, r, domain.NotFound("endpoint"))
			return
		}

		if info, err := os.Stat(filepath.Join(dir, filepath.Clean(r.URL.Path))); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		// Client-side routes have no file behind them, so they get the shell.
		http.ServeFile(w, r, index)
	}
}
