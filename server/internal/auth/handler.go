package auth

import (
	"net/http"

	"github.com/google/uuid"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Sessions interface {
	DisconnectUser(userID uuid.UUID)
	ProfileChanged(user dbgen.User)
}

type Handler struct {
	svc      *Service
	sessions Sessions
}

func NewHandler(svc *Service, sessions Sessions) *Handler {
	return &Handler{svc: svc, sessions: sessions}
}

func (h *Handler) Routes(mux httpx.Router) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
}

type credentials struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User   events.User `json:"user"`
	Tokens TokenPair   `json:"tokens"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	user, tokens, err := h.svc.Register(r.Context(), in.Username, in.Email, in.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, authResponse{User: PublicUser(user), Tokens: tokens})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	user, tokens, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, authResponse{User: PublicUser(user), Tokens: tokens})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var in refreshRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if in.RefreshToken == "" {
		httpx.Error(w, r, domain.Invalid("refresh_token is required"))
		return
	}
	tokens, err := h.svc.Refresh(r.Context(), in.RefreshToken)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tokens)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var in refreshRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Logout(r.Context(), in.RefreshToken); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

type AccountView struct {
	events.User
	Self events.Self `json:"self"`
}

func Account(u dbgen.User) AccountView {
	return AccountView{User: PublicUser(u), Self: SelfOf(u)}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())
	httpx.JSON(w, http.StatusOK, Account(user))
}

func (h *Handler) PatchMe(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())

	var in struct {
		Status       *string `json:"status"`
		CustomStatus *string `json:"custom_status"`
		Bio          *string `json:"bio"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	updated, err := h.svc.UpdateProfile(r.Context(), user.ID, ProfileEdit{
		Status:       in.Status,
		CustomStatus: in.CustomStatus,
		Bio:          in.Bio,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	if h.sessions != nil {
		h.sessions.ProfileChanged(updated)
	}
	httpx.JSON(w, http.StatusOK, Account(updated))
}

func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())

	var in struct {
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), user.ID, in.Password); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if h.sessions != nil {
		h.sessions.DisconnectUser(user.ID)
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func PublicUser(u dbgen.User) events.User {
	return events.User{
		ID:            events.UserID(u.PublicID),
		Username:      u.Username,
		Discriminator: u.Discriminator,
		AvatarKey:     u.AvatarKey,
	}
}
