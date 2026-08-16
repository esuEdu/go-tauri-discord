package guild

import (
	"net/http"

	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
)

func (h *Handler) PublicRoutes(mux httpx.Router) {
	mux.HandleFunc("GET /api/v1/invites/{code}", h.previewInvite)
}

func (h *Handler) previewInvite(w http.ResponseWriter, r *http.Request) {
	invite, err := h.svc.PreviewInvite(r.Context(), r.PathValue("code"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, invite)
}
