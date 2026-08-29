package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/media"
	"github.com/esuEdu/go-tauri-discord/internal/platform/bus"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
	"github.com/esuEdu/go-tauri-discord/internal/storage"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type People interface {
	SetAvatar(ctx context.Context, userID uuid.UUID, key *string) (events.User, error)
	Avatar(ctx context.Context, userID uuid.UUID) (*string, error)
}

type Guilds interface {
	SetIcon(ctx context.Context, actorID, guildID uuid.UUID, key *string) (events.Guild, error)
	Icon(ctx context.Context, guildID uuid.UUID) (*string, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Guild, error)
}

type Attachments interface {
	Attachment(ctx context.Context, id uuid.UUID) (storage.Store, dbgen.Attachment, error)
}

type Handler struct {
	store       storage.Store
	people      People
	guilds      Guilds
	attachments Attachments
	signer      *Signer
	pub         *bus.Publisher
}

func NewHandler(store storage.Store, people People, guilds Guilds, pub *bus.Publisher) *Handler {
	return &Handler{store: store, people: people, guilds: guilds, pub: pub}
}

func (h *Handler) AttachMessages(attachments Attachments, signer *Signer) {
	h.attachments = attachments
	h.signer = signer
}

func (h *Handler) Routes(protected httpx.Router) {
	protected.HandleFunc("PUT /api/v1/users/@me/avatar", h.setAvatar)
	protected.HandleFunc("DELETE /api/v1/users/@me/avatar", h.clearAvatar)
	protected.HandleFunc("PUT /api/v1/guilds/{guildID}/icon", h.setIcon)
	protected.HandleFunc("DELETE /api/v1/guilds/{guildID}/icon", h.clearIcon)
}

func (h *Handler) PublicRoutes(mux httpx.Router) {
	mux.HandleFunc("GET /api/v1/files/{key...}", h.serve)
	mux.HandleFunc("GET /api/v1/attachments/{id}", h.attachment)
}

func (h *Handler) attachment(w http.ResponseWriter, r *http.Request) {
	if h.attachments == nil {
		httpx.Error(w, r, domain.NotFound("attachment"))
		return
	}

	id, err := httpx.PathUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, domain.NotFound("attachment"))
		return
	}

	query := r.URL.Query()
	if !h.signer.Allows(r.URL.Path, query.Get("exp"), query.Get("sig")) {
		httpx.Error(w, r, domain.NotFound("attachment"))
		return
	}

	store, row, err := h.attachments.Attachment(r.Context(), id)
	if err != nil || store == nil {
		httpx.Error(w, r, domain.NotFound("attachment"))
		return
	}

	body, err := store.Open(r.Context(), row.StorageKey)
	if err != nil {
		httpx.Error(w, r, domain.NotFound("attachment"))
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", row.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(row.SizeBytes, 10))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", disposition(row.ContentType, row.Filename))

	if _, err := io.Copy(w, body); err != nil {
		slog.WarnContext(r.Context(), "serving an attachment stopped early",
			"attachment_id", id, "error", err)
	}
}

func disposition(contentType, filename string) string {
	kind := "attachment"
	if _, inline := storage.ExtensionFor(contentType); inline {
		kind = "inline"
	}
	return mime.FormatMediaType(kind, map[string]string{"filename": filename})
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !storage.ValidKey(key) {
		httpx.Error(w, r, domain.NotFound("file"))
		return
	}

	body, err := h.store.Open(r.Context(), key)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrBadKey) {
			httpx.Error(w, r, domain.NotFound("file"))
			return
		}
		httpx.Error(w, r, domain.Internal(err))
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", storage.ContentTypeOf(key))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if _, err := io.Copy(w, body); err != nil {
		slog.WarnContext(r.Context(), "serving a file stopped early", "key", key, "error", err)
	}
}

func (h *Handler) squared(w http.ResponseWriter, r *http.Request, side int) (string, bool) {
	shrunk, contentType, err := media.Square(r.Body, side)
	switch {
	case errors.Is(err, media.ErrTooLarge):
		httpx.Error(w, r, domain.Invalid("that image is larger than %d MB", media.MaxUploadBytes>>20))
		return "", false
	case errors.Is(err, media.ErrTooManyPixels):
		httpx.Error(w, r, domain.Invalid("that image is larger than %d megapixels", media.MaxPixels>>20))
		return "", false
	case errors.Is(err, media.ErrNotAnImage):
		httpx.Error(w, r, domain.Invalid("that file is not a png, jpeg, gif or webp image"))
		return "", false
	case err != nil:
		httpx.Error(w, r, domain.Internal(err))
		return "", false
	}

	key, err := storage.NewKey("avatars", contentType)
	if err != nil {
		httpx.Error(w, r, domain.Internal(err))
		return "", false
	}
	if err := h.store.Put(r.Context(), key, contentType,
		bytes.NewReader(shrunk), int64(len(shrunk))); err != nil {
		httpx.Error(w, r, domain.Internal(err))
		return "", false
	}
	return key, true
}

func (h *Handler) forget(r *http.Request, key *string) {
	if key == nil || *key == "" {
		return
	}
	if err := h.store.Delete(r.Context(), *key); err != nil {
		slog.WarnContext(r.Context(), "the old image was left behind", "key", *key, "error", err)
	}
}

func (h *Handler) setAvatar(w http.ResponseWriter, r *http.Request) {
	userID := auth.MustUserID(r.Context())

	previous, err := h.people.Avatar(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	key, ok := h.squared(w, r, media.AvatarSize)
	if !ok {
		return
	}
	updated, err := h.people.SetAvatar(r.Context(), userID, &key)
	if err != nil {
		h.forget(r, &key)
		httpx.Error(w, r, err)
		return
	}

	h.forget(r, previous)
	h.announceUser(r, updated)
	httpx.JSON(w, http.StatusOK, map[string]string{"avatar_key": key})
}

func (h *Handler) clearAvatar(w http.ResponseWriter, r *http.Request) {
	userID := auth.MustUserID(r.Context())

	previous, err := h.people.Avatar(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	updated, err := h.people.SetAvatar(r.Context(), userID, nil)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.forget(r, previous)
	h.announceUser(r, updated)
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) announceUser(r *http.Request, who events.User) {
	h.pub.ToUser(r.Context(), who.ID, events.EventUserUpdate, who)

	guilds, err := h.guilds.ListForUser(r.Context(), who.ID)
	if err != nil {
		slog.WarnContext(r.Context(), "files: cannot announce a new picture",
			"user_id", who.ID, "error", err)
		return
	}
	for _, g := range guilds {
		h.pub.ToGuild(r.Context(), g.ID, events.EventUserUpdate, who)
	}
}

func (h *Handler) setIcon(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	actorID := auth.MustUserID(r.Context())

	previous, err := h.guilds.Icon(r.Context(), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	key, ok := h.squared(w, r, media.IconSize)
	if !ok {
		return
	}
	updated, err := h.guilds.SetIcon(r.Context(), actorID, guildID, &key)
	if err != nil {
		h.forget(r, &key)
		httpx.Error(w, r, err)
		return
	}

	h.forget(r, previous)
	h.pub.ToGuild(r.Context(), guildID, events.EventGuildUpdate, updated)
	httpx.JSON(w, http.StatusOK, map[string]string{"icon_key": key})
}

func (h *Handler) clearIcon(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	actorID := auth.MustUserID(r.Context())

	previous, err := h.guilds.Icon(r.Context(), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	updated, err := h.guilds.SetIcon(r.Context(), actorID, guildID, nil)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.forget(r, previous)
	h.pub.ToGuild(r.Context(), guildID, events.EventGuildUpdate, updated)
	httpx.JSON(w, http.StatusNoContent, nil)
}
