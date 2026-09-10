package guild

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/bus"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Handler struct {
	svc   *Service
	pub   *bus.Publisher
	rooms Rooms
}

type Rooms interface {
	JoinedGuild(userID, guildID uuid.UUID)
	LeftGuild(userID, guildID uuid.UUID)
	ClosedChannel(guildID, channelID uuid.UUID)
	Statuses(userIDs []uuid.UUID) map[uuid.UUID]events.PresenceUpdate
}

func NewHandler(svc *Service, pub *bus.Publisher, rooms Rooms) *Handler {
	return &Handler{svc: svc, pub: pub, rooms: rooms}
}

func (h *Handler) joined(userID, guildID uuid.UUID) {
	if h.rooms != nil {
		h.rooms.JoinedGuild(userID, guildID)
	}
}

func (h *Handler) closed(guildID, channelID uuid.UUID) {
	if h.rooms != nil {
		h.rooms.ClosedChannel(guildID, channelID)
	}
}

func seen(status string) string {
	if status == "" {
		return string(domain.StatusOffline)
	}
	return status
}

func (h *Handler) statuses(userIDs []uuid.UUID) map[uuid.UUID]events.PresenceUpdate {
	if h.rooms == nil {
		return nil
	}
	return h.rooms.Statuses(userIDs)
}

func (h *Handler) removed(r *http.Request, guildID, userID uuid.UUID, banned bool) {
	if h.rooms != nil {
		h.rooms.LeftGuild(userID, guildID)
	}
	who, err := h.svc.Who(r.Context(), userID)
	if err != nil {
		return
	}
	departure := events.GuildRemoval{GuildID: guildID, UserID: who, Banned: banned}
	h.pub.ToGuild(r.Context(), guildID, events.EventGuildMemberRemove, departure)
	h.pub.ToUser(r.Context(), userID, events.EventGuildRemove, departure)
}

func (h *Handler) Routes(mux httpx.Router) {
	mux.HandleFunc("POST /api/v1/guilds", h.create)
	mux.HandleFunc("GET /api/v1/guilds", h.list)
	mux.HandleFunc("PUT /api/v1/guilds/order", h.reorder)
	mux.HandleFunc("POST /api/v1/guilds/{guildID}/invites", h.createInvite)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/invites", h.listInvites)
	mux.HandleFunc("POST /api/v1/invites/{code}", h.redeemInvite)
	mux.HandleFunc("DELETE /api/v1/invites/{code}", h.revokeInvite)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/members", h.members)
	mux.HandleFunc("DELETE /api/v1/guilds/{guildID}/members/@me", h.leave)
	mux.HandleFunc("DELETE /api/v1/guilds/{guildID}/members/{userID}", h.kick)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/bans", h.listBans)
	mux.HandleFunc("PUT /api/v1/guilds/{guildID}/bans/{userID}", h.ban)
	mux.HandleFunc("DELETE /api/v1/guilds/{guildID}/bans/{userID}", h.unban)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/channels", h.listChannels)
	mux.HandleFunc("POST /api/v1/guilds/{guildID}/channels", h.createChannel)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/roles", h.listRoles)
	mux.HandleFunc("POST /api/v1/guilds/{guildID}/roles", h.createRole)
	mux.HandleFunc("PATCH /api/v1/roles/{roleID}", h.updateRole)
	mux.HandleFunc("DELETE /api/v1/roles/{roleID}", h.deleteRole)
	mux.HandleFunc("GET /api/v1/guilds/{guildID}/members/{userID}/roles", h.memberRoles)
	mux.HandleFunc("PUT /api/v1/guilds/{guildID}/members/{userID}/roles/{roleID}", h.assignRole)
	mux.HandleFunc("DELETE /api/v1/guilds/{guildID}/members/{userID}/roles/{roleID}", h.unassignRole)
	mux.HandleFunc("PATCH /api/v1/guilds/{guildID}", h.updateGuild)
	mux.HandleFunc("PATCH /api/v1/guilds/{guildID}/members/{userID}", h.setNickname)
	mux.HandleFunc("PATCH /api/v1/channels/{channelID}", h.updateChannel)
	mux.HandleFunc("DELETE /api/v1/channels/{channelID}", h.deleteChannel)
	mux.HandleFunc("PATCH /api/v1/channels/{channelID}/position", h.moveChannel)
	mux.HandleFunc("GET /api/v1/channels/{channelID}/overwrites", h.listOverwrites)
	mux.HandleFunc("PUT /api/v1/channels/{channelID}/overwrites/{targetID}", h.setOverwrite)
	mux.HandleFunc("DELETE /api/v1/channels/{channelID}/overwrites/{targetID}", h.clearOverwrite)
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

	public, err := h.svc.PublicGuild(r.Context(), g)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.joined(userID, g.ID)
	h.pub.ToUser(r.Context(), userID, events.EventGuildCreate, public)
	httpx.JSON(w, http.StatusCreated, public)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	guilds, err := h.svc.ListForUser(r.Context(), auth.MustUserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	public, err := mapEach(r.Context(), guilds, h.svc.PublicGuild)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, public)
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

	public, err := h.svc.PublicGuild(r.Context(), guild)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.joined(userID, guild.ID)
	if member, err := h.svc.NewMember(r.Context(), guild.ID, userID); err == nil {
		h.pub.ToGuild(r.Context(), guild.ID, events.EventGuildMemberAdd, member)
	}
	h.pub.ToUser(r.Context(), userID, events.EventGuildCreate, public)
	httpx.JSON(w, http.StatusOK, public)
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
		UserID        events.UserID `json:"user_id"`
		Username      string        `json:"username"`
		Discriminator string        `json:"discriminator"`
		Nickname      *string       `json:"nickname"`
		AvatarKey     *string       `json:"avatar_key"`
		Status        string        `json:"status"`
		CustomStatus  *string       `json:"custom_status"`
	}
	ids := make([]uuid.UUID, len(rows))
	for i, m := range rows {
		ids[i] = m.UserID
	}
	presence := h.statuses(ids)

	out := make([]member, len(rows))
	for i, m := range rows {
		out[i] = member{
			UserID:        events.UserID(m.PublicID),
			Username:      m.Username,
			Discriminator: m.Discriminator,
			Nickname:      m.Nickname,
			AvatarKey:     m.AvatarKey,
			Status:        seen(presence[m.UserID].Status),
			CustomStatus:  presence[m.UserID].CustomStatus,
		}
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) kick(w http.ResponseWriter, r *http.Request) {
	guildID, memberID, err := h.guildAndMember(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Kick(r.Context(), auth.MustUserID(r.Context()), guildID, memberID); err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.removed(r, guildID, memberID, false)
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GuildIDs []uuid.UUID `json:"guild_ids"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	userID := auth.MustUserID(r.Context())
	if err := h.svc.Reorder(r.Context(), userID, in.GuildIDs); err != nil {
		httpx.Error(w, r, err)
		return
	}

	guilds, err := h.svc.ListForUser(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	public, err := mapEach(r.Context(), guilds, h.svc.PublicGuild)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, public)
}

func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	userID := auth.MustUserID(r.Context())
	if err := h.svc.Leave(r.Context(), userID, guildID); err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.removed(r, guildID, userID, false)
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ban(w http.ResponseWriter, r *http.Request) {
	guildID, memberID, err := h.guildAndMember(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Reason *string `json:"reason"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	ban, err := h.svc.Ban(r.Context(), auth.MustUserID(r.Context()), guildID, memberID, in.Reason)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.removed(r, guildID, memberID, true)
	httpx.JSON(w, http.StatusCreated, ban)
}

func (h *Handler) unban(w http.ResponseWriter, r *http.Request) {
	guildID, memberID, err := h.guildAndMember(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Unban(r.Context(), auth.MustUserID(r.Context()), guildID, memberID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) listBans(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	bans, err := h.svc.ListBans(r.Context(), auth.MustUserID(r.Context()), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, bans)
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
		Name     string     `json:"name"`
		Kind     string     `json:"kind"`
		Position int32      `json:"position"`
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	ch, err := h.svc.CreateChannel(r.Context(), auth.MustUserID(r.Context()), guildID, in.Name, in.Kind, in.Position, in.ParentID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToGuild(r.Context(), guildID, events.EventChannelCreate, PublicChannel(ch))
	httpx.JSON(w, http.StatusCreated, PublicChannel(ch))
}

type optionalParent struct {
	given bool
	id    *uuid.UUID
}

func (o *optionalParent) UnmarshalJSON(b []byte) error {
	o.given = true
	if string(b) == "null" {
		o.id = nil
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(b, &id); err != nil {
		return domain.Invalid("parent_id must be a channel id or null")
	}
	o.id = &id
	return nil
}

func (h *Handler) updateGuild(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Name *string `json:"name"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	updated, err := h.svc.Update(r.Context(), auth.MustUserID(r.Context()), guildID, in.Name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	public, err := h.svc.PublicGuild(r.Context(), updated)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToGuild(r.Context(), guildID, events.EventGuildUpdate, public)
	httpx.JSON(w, http.StatusOK, public)
}

func (h *Handler) memberTarget(r *http.Request, me uuid.UUID) (uuid.UUID, error) {
	if r.PathValue("userID") == "@me" {
		return me, nil
	}
	return h.svc.Whom(r.Context(), events.UserID(r.PathValue("userID")))
}

func (h *Handler) setNickname(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	targetID, err := h.memberTarget(r, auth.MustUserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Nickname *string `json:"nickname"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	member, err := h.svc.SetNickname(
		r.Context(), auth.MustUserID(r.Context()), guildID, targetID, in.Nickname,
	)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToGuild(r.Context(), guildID, events.EventGuildMemberUpdate, member)
	httpx.JSON(w, http.StatusOK, member)
}

func (h *Handler) updateChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Name  *string       `json:"name"`
		Topic optionalTopic `json:"topic"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	updated, err := h.svc.UpdateChannel(
		r.Context(), auth.MustUserID(r.Context()), channelID,
		in.Name, in.Topic.text, in.Topic.given && in.Topic.text == nil,
	)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.pub.ToGuild(r.Context(), updated.GuildID, events.EventChannelUpdate, PublicChannel(updated))
	httpx.JSON(w, http.StatusOK, PublicChannel(updated))
}

func (h *Handler) deleteChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	gone, err := h.svc.DeleteChannel(r.Context(), auth.MustUserID(r.Context()), channelID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	h.closed(gone.GuildID, gone.ID)
	h.pub.ToGuild(r.Context(), gone.GuildID, events.EventChannelDelete, PublicChannel(gone))
	w.WriteHeader(http.StatusNoContent)
}

type optionalTopic struct {
	given bool
	text  *string
}

func (o *optionalTopic) UnmarshalJSON(b []byte) error {
	o.given = true
	if string(b) == "null" {
		o.text = nil
		return nil
	}
	var text string
	if err := json.Unmarshal(b, &text); err != nil {
		return domain.Invalid("topic must be text or null")
	}
	o.text = &text
	return nil
}

func (h *Handler) moveChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Position int            `json:"position"`
		ParentID optionalParent `json:"parent_id"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	moved, err := h.svc.MoveChannel(
		r.Context(), auth.MustUserID(r.Context()), channelID,
		in.Position, in.ParentID.id, in.ParentID.given,
	)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	for _, ch := range moved {
		h.pub.ToGuild(r.Context(), ch.GuildID, events.EventChannelUpdate, PublicChannel(ch))
	}
	httpx.JSON(w, http.StatusOK, mapSlice(moved, PublicChannel))
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	roles, err := h.svc.ListRoles(r.Context(), auth.MustUserID(r.Context()), guildID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mapSlice(roles, PublicRole))
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	guildID, err := httpx.PathUUID(r, "guildID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Name        string `json:"name"`
		Permissions int64  `json:"permissions"`
		Position    *int32 `json:"position"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	role, err := h.svc.CreateRole(r.Context(), auth.MustUserID(r.Context()), guildID,
		in.Name, domain.Permission(in.Permissions), in.Position)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, PublicRole(role))
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	roleID, err := httpx.PathUUID(r, "roleID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		Name        *string `json:"name"`
		Permissions *int64  `json:"permissions"`
		Position    *int32  `json:"position"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	var perms *domain.Permission
	if in.Permissions != nil {
		p := domain.Permission(*in.Permissions)
		perms = &p
	}

	role, err := h.svc.UpdateRole(r.Context(), auth.MustUserID(r.Context()), roleID, in.Name, perms, in.Position)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, PublicRole(role))
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	roleID, err := httpx.PathUUID(r, "roleID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.DeleteRole(r.Context(), auth.MustUserID(r.Context()), roleID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) memberRoles(w http.ResponseWriter, r *http.Request) {
	guildID, memberID, err := h.guildAndMember(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	roles, err := h.svc.MemberRoles(r.Context(), auth.MustUserID(r.Context()), guildID, memberID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mapSlice(roles, PublicRole))
}

func (h *Handler) assignRole(w http.ResponseWriter, r *http.Request) {
	guildID, memberID, roleID, err := h.guildMemberAndRole(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.AssignRole(r.Context(), auth.MustUserID(r.Context()), guildID, memberID, roleID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) unassignRole(w http.ResponseWriter, r *http.Request) {
	guildID, memberID, roleID, err := h.guildMemberAndRole(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.UnassignRole(r.Context(), auth.MustUserID(r.Context()), guildID, memberID, roleID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) listOverwrites(w http.ResponseWriter, r *http.Request) {
	channelID, err := httpx.PathUUID(r, "channelID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	rows, err := h.svc.ChannelOverwrites(r.Context(), auth.MustUserID(r.Context()), channelID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	public, err := mapEach(r.Context(), rows, h.svc.PublicOverwrite)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, public)
}

func (h *Handler) setOverwrite(w http.ResponseWriter, r *http.Request) {
	channelID, targetID, err := h.channelAndTarget(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		TargetType string `json:"target_type"`
		Allow      int64  `json:"allow"`
		Deny       int64  `json:"deny"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}

	if err := h.svc.SetChannelOverwrite(r.Context(), auth.MustUserID(r.Context()), channelID, targetID,
		in.TargetType, domain.Permission(in.Allow), domain.Permission(in.Deny)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) clearOverwrite(w http.ResponseWriter, r *http.Request) {
	channelID, targetID, err := h.channelAndTarget(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.ClearChannelOverwrite(r.Context(), auth.MustUserID(r.Context()), channelID, targetID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) guildAndMember(r *http.Request) (guildID, memberID uuid.UUID, err error) {
	if guildID, err = httpx.PathUUID(r, "guildID"); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if memberID, err = h.svc.Whom(r.Context(), events.UserID(r.PathValue("userID"))); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return guildID, memberID, nil
}

func (h *Handler) guildMemberAndRole(r *http.Request) (guildID, memberID, roleID uuid.UUID, err error) {
	if guildID, memberID, err = h.guildAndMember(r); err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	if roleID, err = httpx.PathUUID(r, "roleID"); err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return guildID, memberID, roleID, nil
}

func (h *Handler) channelAndTarget(r *http.Request) (channelID, targetID uuid.UUID, err error) {
	if channelID, err = httpx.PathUUID(r, "channelID"); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	raw := r.PathValue("targetID")
	if parsed, err := uuid.Parse(raw); err == nil {
		return channelID, parsed, nil
	}
	targetID, err = h.svc.Whom(r.Context(), events.UserID(raw))
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return channelID, targetID, nil
}

func mapSlice[T any, R any](in []T, fn func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

func mapEach[T any, R any](ctx context.Context, in []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	out := make([]R, len(in))
	for i, v := range in {
		mapped, err := fn(ctx, v)
		if err != nil {
			return nil, err
		}
		out[i] = mapped
	}
	return out, nil
}
