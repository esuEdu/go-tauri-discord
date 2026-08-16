package guild

import (
	"net/http"
	"time"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	"github.com/esuEdu/go-tauri-discord/internal/platform/bus"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Handler struct {
	svc *Service
	pub *bus.Publisher
}

func NewHandler(svc *Service, pub *bus.Publisher) *Handler {
	return &Handler{svc: svc, pub: pub}
}

func (h *Handler) Routes(mux httpx.Router) {
	mux.HandleFunc("POST /api/v1/guilds", h.create)
	mux.HandleFunc("GET /api/v1/guilds", h.list)
	mux.HandleFunc("POST /api/v1/guilds/{guildID}/invites", h.createInvite)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/invites", h.listInvites)
	mux.HandleFunc("POST /api/v1/invites/{code}", h.redeemInvite)
	mux.HandleFunc("DELETE /api/v1/invites/{code}", h.revokeInvite)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/members", h.members)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/channels", h.listChannels)
	mux.HandleFunc("POST /api/v1/guilds/{guildID}/channels", h.createChannel)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	userID := auth.MustUserID(r.Context())

	g, err := h.svc.Create(r.Context(), userID, in.Name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToUser(r.Context(), userID, events.EventGuildCreate, PublicGuild(g))
	httpx.JSON(w, http.StatusCreated, PublicGuild(g))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	guilds, err := h.svc.ListForUser(r.Context(), auth.MustUserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mapSlice(guilds, PublicGuild))
}

func (h *Handler) createInvite(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		MaxUses        *int32 `json:"max_uses"`
		ExpiresInHours *int   `json:"expires_in_hours"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	var expiresIn *time.Duration
	if in.ExpiresInHours != nil {
		d := time.Duration(*in.ExpiresInHours) * time.Hour
		expiresIn = &d
	}

	invite, err := h.svc.CreateInvite(r.Context(), auth.MustUserID(r.Context()), guildID, in.MaxUses, expiresIn)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, invite)
}

func (h *Handler) listInvites(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	invites, err := h.svc.ListInvites(r.Context(), auth.MustUserID(r.Context()), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, invites)
}

func (h *Handler) redeemInvite(w http.ResponseWriter, r *http.Request) {
	userID := auth.MustUserID(r.Context())
	guild, err := h.svc.RedeemInvite(r.Context(), userID, r.PathValue("code"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToUser(r.Context(), userID, events.EventGuildCreate, PublicGuild(guild))
	httpx.JSON(w, http.StatusOK, PublicGuild(guild))
}

func (h *Handler) revokeInvite(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeInvite(r.Context(), auth.MustUserID(r.Context()), r.PathValue("code")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	rows, err := h.svc.Members(r.Context(), auth.MustUserID(r.Context()), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	type member struct {
		UserID    string  `json:"user_id"`
		Username  string  `json:"username"`
		Nickname  *string `json:"nickname"`
		AvatarKey *string `json:"avatar_key"`
	}
	out := make([]member, len(rows))
	for i, m := range rows {
		out[i] = member{
			UserID:    m.UserID.String(),
			Username:  m.Username,
			Nickname:  m.Nickname,
			AvatarKey: m.AvatarKey,
		}
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	channels, err := h.svc.ListChannels(r.Context(), auth.MustUserID(r.Context()), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mapSlice(channels, PublicChannel))
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		Position int32  `json:"position"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	ch, err := h.svc.CreateChannel(r.Context(), auth.MustUserID(r.Context()), guildID, in.Name, in.Kind, in.Position)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToGuild(r.Context(), guildID, events.EventChannelCreate, PublicChannel(ch))
	httpx.JSON(w, http.StatusCreated, PublicChannel(ch))
}

func mapSlice[T any, R any](in []T, fn func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}
