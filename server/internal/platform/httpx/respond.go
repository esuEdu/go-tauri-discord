package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

const maxBodyBytes = 1 << 20

type errorBody struct {
	Error string `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	if v == nil {
		w.WriteHeader(status)
		return
	}
	buf, err := json.Marshal(v)
	if err != nil {
		slog.Error("marshal response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	kind := domain.KindOf(err)
	status := statusFor(kind)

	msg := "internal error"
	if de, ok := errors.AsType[*domain.Error](err); ok && kind != domain.KindInternal {
		msg = de.Msg
	}

	if status >= 500 {
		slog.ErrorContext(r.Context(), "request failed",
			"method", r.Method, "path", r.URL.Path, "error", err)
	}
	JSON(w, status, errorBody{Error: msg})
}

func statusFor(k domain.Kind) int {
	switch k {
	case domain.KindInvalid:
		return http.StatusBadRequest
	case domain.KindUnauthorized:
		return http.StatusUnauthorized
	case domain.KindForbidden:
		return http.StatusForbidden
	case domain.KindNotFound:
		return http.StatusNotFound
	case domain.KindConflict:
		return http.StatusConflict
	case domain.KindRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return domain.Invalid("malformed request body: %v", err)
	}

	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return domain.Invalid("request body must contain a single JSON object")
	}
	return nil
}
