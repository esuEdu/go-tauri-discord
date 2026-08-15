package message

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
)

func parseUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, domain.Invalid("message_id must be a uuid")
	}
	return id, nil
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(mux httpx.Router) {
	mux.HandleFunc("GET /api/v1/channels/{channelID}/messages", h.history)
	mux.HandleFunc("POST /api/v1/channels/{channelID}/messages", h.create)
	mux.HandleFunc("PATCH /api/v1/messages/{messageID}", h.edit)
	mux.HandleFunc("DELETE /api/v1/messages/{messageID}", h.delete)
	mux.HandleFunc("POST /api/v1/channels/{channelID}/typing", h.typing)
	mux.HandleFunc("PUT /api/v1/channels/{channelID}/read", h.markRead)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	before, err := httpx.QueryUUID(r, "before")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	limit, err := httpx.QueryInt(r, "limit", DefaultPageSize, 1, MaxPageSize)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	msgs, err := h.svc.History(r.Context(), auth.MustUserID(r.Context()), channelID, before, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, msgs)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	msg, err := h.svc.Create(r.Context(), auth.MustUserID(r.Context()), channelID, in.Content)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, msg)
}

func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	messageID, err := httpx.PathUUID(r, "messageID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	msg, err := h.svc.Edit(r.Context(), auth.MustUserID(r.Context()), messageID, in.Content)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, msg)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	messageID, err := httpx.PathUUID(r, "messageID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), auth.MustUserID(r.Context()), messageID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) typing(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Typing(r.Context(), auth.MustUserID(r.Context()), channelID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		MessageID string `json:"message_id"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	messageID, err := parseUUID(in.MessageID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.MarkRead(r.Context(), auth.MustUserID(r.Context()), channelID, messageID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
