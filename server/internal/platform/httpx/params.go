package httpx

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := r.PathValue(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, domain.Invalid("%s must be a uuid", name)
	}
	return id, nil
}

func QueryUUID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, domain.Invalid("%s must be a uuid", name)
	}
	return &id, nil
}

func QueryInt(r *http.Request, name string, fallback, minimum, maximum int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, domain.Invalid("%s must be an integer", name)
	}
	return min(max(n, minimum), maximum), nil
}
