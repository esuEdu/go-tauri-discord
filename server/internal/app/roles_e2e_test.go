//go:build e2e

package app_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func perm(p domain.Permission) int64 { return int64(p) }

func (h *harness) roles(guildID uuid.UUID) []events.Role {
	h.t.Helper()
	var out []events.Role
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/roles", http.StatusOK, nil, &out)
	return out
}

func (h *harness) everyone(guildID uuid.UUID) events.Role {
	h.t.Helper()
	for _, r := range h.roles(guildID) {
		if r.IsDefault {
			return r
		}
	}
	h.t.Fatal("guild has no @everyone role")
	return events.Role{}
}

func (h *harness) createRole(guildID uuid.UUID, body any) events.Role {
	h.t.Helper()
	var out events.Role
	h.mustDo(http.MethodPost, "/api/v1/guilds/"+guildID.String()+"/roles",
		http.StatusCreated, body, &out)
	return out
}

func (h *harness) inviteMember(guildID uuid.UUID) *harness {
	h.t.Helper()
	invite := h.createInvite(guildID, map[string]any{})
	member := h.newUser()
	member.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)
	return member
}

func memberRolesPath(guildID, memberID, roleID uuid.UUID) string {
	return "/api/v1/guilds/" + guildID.String() + "/members/" + memberID.String() +
		"/roles/" + roleID.String()
}

func TestRoleLifecycle(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Roles")

	if got := len(owner.roles(guild.ID)); got != 1 {
		t.Fatalf("a fresh guild has %d roles, want only @everyone", got)
	}

	role := owner.createRole(guild.ID, map[string]any{
		"name":        "Moderator",
		"permissions": perm(domain.PermManageMessages | domain.PermKickMembers),
		"position":    5,
	})
	if role.Name != "Moderator" || role.Position != 5 || role.IsDefault {
		t.Fatalf("created role = %+v", role)
	}

	var patched events.Role
	owner.mustDo(http.MethodPatch, "/api/v1/roles/"+role.ID.String(), http.StatusOK, map[string]any{
		"name":        "Helper",
		"permissions": perm(domain.PermManageMessages),
	}, &patched)
	if patched.Name != "Helper" {
		t.Errorf("name = %q, want Helper", patched.Name)
	}
	if patched.Permissions != perm(domain.PermManageMessages) {
		t.Errorf("permissions = %d, want %d", patched.Permissions, perm(domain.PermManageMessages))
	}
	if patched.Position != 5 {
		t.Errorf("position = %d, want 5: an untouched field must not move", patched.Position)
	}

	owner.mustDo(http.MethodDelete, "/api/v1/roles/"+role.ID.String(), http.StatusNoContent, nil, nil)
	for _, r := range owner.roles(guild.ID) {
		if r.ID == role.ID {
			t.Error("the deleted role is still listed")
		}
	}
}

func TestAssigningARoleChangesTheNextRequest(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Promotion")
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	newChannel := map[string]any{"name": "created-by-a-mod", "kind": "text"}
	channelsPath := "/api/v1/guilds/" + guild.ID.String() + "/channels"

	member.mustDo(http.MethodPost, channelsPath, http.StatusForbidden, newChannel, nil)

	role := owner.createRole(guild.ID, map[string]any{
		"name":        "Channel Manager",
		"permissions": perm(domain.PermManageChannels),
		"position":    3,
	})
	owner.mustDo(http.MethodPut, memberRolesPath(guild.ID, memberID, role.ID), http.StatusNoContent, nil, nil)

	member.mustDo(http.MethodPost, channelsPath, http.StatusCreated, newChannel, nil)

	owner.mustDo(http.MethodDelete, memberRolesPath(guild.ID, memberID, role.ID), http.StatusNoContent, nil, nil)

	member.mustDo(http.MethodPost, channelsPath, http.StatusForbidden, newChannel, nil)
}

func TestHierarchyStopsPrivilegeEscalation(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Hierarchy")
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	mod := owner.createRole(guild.ID, map[string]any{
		"name":        "Moderator",
		"permissions": perm(domain.PermManageRoles | domain.PermKickMembers),
		"position":    5,
	})
	owner.mustDo(http.MethodPut, memberRolesPath(guild.ID, memberID, mod.ID), http.StatusNoContent, nil, nil)

	above := owner.createRole(guild.ID, map[string]any{
		"name":        "Admin",
		"permissions": perm(domain.PermAdministrator),
		"position":    9,
	})

	rolesPath := "/api/v1/guilds/" + guild.ID.String() + "/roles"

	member.mustDo(http.MethodPost, rolesPath, http.StatusForbidden, map[string]any{
		"name": "Peer", "permissions": perm(domain.PermKickMembers), "position": 5,
	}, nil)

	member.mustDo(http.MethodPost, rolesPath, http.StatusForbidden, map[string]any{
		"name": "Higher", "permissions": perm(domain.PermKickMembers), "position": 6,
	}, nil)

	member.mustDo(http.MethodPost, rolesPath, http.StatusForbidden, map[string]any{
		"name": "Sneaky Admin", "permissions": perm(domain.PermAdministrator), "position": 1,
	}, nil)

	member.mustDo(http.MethodPost, rolesPath, http.StatusForbidden, map[string]any{
		"name": "Sneaky Banner", "permissions": perm(domain.PermBanMembers), "position": 1,
	}, nil)

	member.mustDo(http.MethodPatch, "/api/v1/roles/"+above.ID.String(), http.StatusForbidden,
		map[string]any{"name": "Demoted"}, nil)
	member.mustDo(http.MethodDelete, "/api/v1/roles/"+above.ID.String(), http.StatusForbidden, nil, nil)
	member.mustDo(http.MethodPut, memberRolesPath(guild.ID, memberID, above.ID),
		http.StatusForbidden, nil, nil)

	member.mustDo(http.MethodPatch, "/api/v1/roles/"+mod.ID.String(), http.StatusForbidden,
		map[string]any{"name": "Self Promotion"}, nil)

	allowed := member.createRole(guild.ID, map[string]any{
		"name": "Trainee", "permissions": perm(domain.PermKickMembers), "position": 1,
	})
	if allowed.Position != 1 {
		t.Errorf("position = %d, want 1", allowed.Position)
	}
}

func TestEveryoneRoleIsProtected(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Everyone")
	everyone := owner.everyone(guild.ID)
	path := "/api/v1/roles/" + everyone.ID.String()

	owner.mustDo(http.MethodDelete, path, http.StatusBadRequest, nil, nil)
	owner.mustDo(http.MethodPatch, path, http.StatusBadRequest, map[string]any{"name": "@nobody"}, nil)
	owner.mustDo(http.MethodPatch, path, http.StatusBadRequest, map[string]any{"position": 4}, nil)

	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)
	owner.mustDo(http.MethodPut, memberRolesPath(guild.ID, memberID, everyone.ID),
		http.StatusBadRequest, nil, nil)

	var updated events.Role
	owner.mustDo(http.MethodPatch, path, http.StatusOK, map[string]any{
		"permissions": perm(domain.DefaultEveryonePermissions &^ domain.PermSendMessages),
	}, &updated)
	if updated.Permissions&perm(domain.PermSendMessages) != 0 {
		t.Fatal("@everyone kept SendMessages after it was revoked")
	}

	text, _ := owner.textAndVoice(guild.ID)
	member.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusForbidden, map[string]string{"content": "muted"}, nil)
}

func TestChannelOverwriteMakesAChannelPrivateOverHTTP(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Private Channel")
	member := owner.inviteMember(guild.ID)
	text, _ := owner.textAndVoice(guild.ID)
	everyone := owner.everyone(guild.ID)

	if !seesChannel(member.listChannels(guild.ID), text) {
		t.Fatal("the member cannot see the channel before any overwrite")
	}

	overwritePath := "/api/v1/channels/" + text.String() + "/overwrites/" + everyone.ID.String()
	owner.mustDo(http.MethodPut, overwritePath, http.StatusNoContent, map[string]any{
		"target_type": "role",
		"deny":        perm(domain.PermViewChannel),
	}, nil)

	if seesChannel(member.listChannels(guild.ID), text) {
		t.Error("the channel is still listed after ViewChannel was denied")
	}
	member.mustDo(http.MethodGet, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusForbidden, nil, nil)
	member.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusForbidden, map[string]string{"content": "hidden"}, nil)

	if !seesChannel(owner.listChannels(guild.ID), text) {
		t.Error("the owner lost a channel to an overwrite")
	}

	owner.mustDo(http.MethodDelete, overwritePath, http.StatusNoContent, nil, nil)
	if !seesChannel(member.listChannels(guild.ID), text) {
		t.Error("clearing the overwrite did not restore the channel")
	}
}

func TestMemberOverwriteBeatsTheRoleOverwrite(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Overwrites")
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)
	text, _ := owner.textAndVoice(guild.ID)
	everyone := owner.everyone(guild.ID)

	owner.mustDo(http.MethodPut,
		"/api/v1/channels/"+text.String()+"/overwrites/"+everyone.ID.String(),
		http.StatusNoContent, map[string]any{
			"target_type": "role",
			"deny":        perm(domain.PermSendMessages),
		}, nil)

	member.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusForbidden, map[string]string{"content": "denied"}, nil)

	owner.mustDo(http.MethodPut,
		"/api/v1/channels/"+text.String()+"/overwrites/"+memberID.String(),
		http.StatusNoContent, map[string]any{
			"target_type": "member",
			"allow":       perm(domain.PermSendMessages),
		}, nil)

	member.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "allowed again"}, nil)

	var rows []events.Overwrite
	owner.mustDo(http.MethodGet, "/api/v1/channels/"+text.String()+"/overwrites",
		http.StatusOK, nil, &rows)
	if len(rows) != 2 {
		t.Fatalf("listed %d overwrites, want 2", len(rows))
	}
}

func TestOverwritesRefuseNonsense(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Bad Overwrites")
	text, _ := owner.textAndVoice(guild.ID)
	everyone := owner.everyone(guild.ID)
	path := "/api/v1/channels/" + text.String() + "/overwrites/" + everyone.ID.String()

	owner.mustDo(http.MethodPut, path, http.StatusBadRequest, map[string]any{
		"target_type": "role",
		"allow":       perm(domain.PermSendMessages),
		"deny":        perm(domain.PermSendMessages),
	}, nil)

	owner.mustDo(http.MethodPut, path, http.StatusBadRequest, map[string]any{
		"target_type": "role",
		"allow":       perm(domain.PermAdministrator),
	}, nil)

	owner.mustDo(http.MethodPut, path, http.StatusBadRequest, map[string]any{
		"target_type": "sorcerer",
		"deny":        perm(domain.PermSendMessages),
	}, nil)

	owner.mustDo(http.MethodPut, path, http.StatusBadRequest, map[string]any{
		"target_type": "role",
		"deny":        int64(1) << 40,
	}, nil)
}

func TestRoleManagementNeedsMembershipAndPermission(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Guarded")
	role := owner.createRole(guild.ID, map[string]any{
		"name": "Staff", "permissions": perm(domain.PermKickMembers), "position": 2,
	})

	outsider := owner.newUser()
	outsider.mustDo(http.MethodGet, "/api/v1/guilds/"+guild.ID.String()+"/roles",
		http.StatusNotFound, nil, nil)
	outsider.mustDo(http.MethodPost, "/api/v1/guilds/"+guild.ID.String()+"/roles",
		http.StatusNotFound, map[string]any{"name": "Intruder", "permissions": 0}, nil)
	outsider.mustDo(http.MethodPatch, "/api/v1/roles/"+role.ID.String(),
		http.StatusNotFound, map[string]any{"name": "Mine Now"}, nil)

	member := owner.inviteMember(guild.ID)
	if got := len(member.roles(guild.ID)); got != 2 {
		t.Errorf("a plain member lists %d roles, want 2", got)
	}
	member.mustDo(http.MethodPost, "/api/v1/guilds/"+guild.ID.String()+"/roles",
		http.StatusForbidden, map[string]any{"name": "Nope", "permissions": 0}, nil)
	member.mustDo(http.MethodDelete, "/api/v1/roles/"+role.ID.String(), http.StatusForbidden, nil, nil)
}

func TestRevokingViewChannelTakesEffectOnTheNextRequest(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Revocation")
	member := owner.inviteMember(guild.ID)
	text, _ := owner.textAndVoice(guild.ID)
	everyone := owner.everyone(guild.ID)

	member.mustDo(http.MethodGet, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusOK, nil, nil)

	owner.mustDo(http.MethodPut,
		"/api/v1/channels/"+text.String()+"/overwrites/"+everyone.ID.String(),
		http.StatusNoContent, map[string]any{
			"target_type": "role",
			"deny":        perm(domain.PermViewChannel),
		}, nil)

	member.mustDo(http.MethodGet, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusForbidden, nil, nil)
}

func TestReadyHidesAChannelTheMemberCannotView(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Ready Hides")
	member := owner.inviteMember(guild.ID)
	text, _ := owner.textAndVoice(guild.ID)
	everyone := owner.everyone(guild.ID)

	owner.mustDo(http.MethodPut,
		"/api/v1/channels/"+text.String()+"/overwrites/"+everyone.ID.String(),
		http.StatusNoContent, map[string]any{
			"target_type": "role",
			"deny":        perm(domain.PermViewChannel),
		}, nil)

	sock := member.dial()
	ready := sock.identify(member.token)
	for _, c := range ready.Channels {
		if c.ID == text {
			t.Fatal("READY lists a channel the member cannot view")
		}
	}
}

func (h *harness) post(channelID uuid.UUID, content string) {
	h.t.Helper()
	h.mustDo(http.MethodPost, "/api/v1/channels/"+channelID.String()+"/messages",
		http.StatusCreated, map[string]string{"content": content}, nil)
}

func (h *harness) newTextChannel(guildID uuid.UUID, name string) uuid.UUID {
	h.t.Helper()
	var ch events.Channel
	h.mustDo(http.MethodPost, "/api/v1/guilds/"+guildID.String()+"/channels",
		http.StatusCreated, map[string]any{"name": name, "kind": "text"}, &ch)
	return ch.ID
}

func (h *harness) denyView(channelID, targetID uuid.UUID, targetType string) {
	h.t.Helper()
	h.mustDo(http.MethodPut,
		"/api/v1/channels/"+channelID.String()+"/overwrites/"+targetID.String(),
		http.StatusNoContent, map[string]any{
			"target_type": targetType,
			"deny":        perm(domain.PermViewChannel),
		}, nil)
}

func (s *socket) nextMessage() events.Message {
	s.t.Helper()
	frame := s.readEvent(events.EventMessageCreate)
	var msg events.Message
	decode(s.t, frame.D, &msg)
	return msg
}

func TestGatewayWithholdsAHiddenChannelsMessages(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Withheld")
	secret, _ := owner.textAndVoice(guild.ID)
	open := owner.newTextChannel(guild.ID, "open")
	member := owner.inviteMember(guild.ID)
	everyone := owner.everyone(guild.ID)

	owner.denyView(secret, everyone.ID, "role")

	sock := member.dial()
	sock.identify(member.token)

	owner.post(secret, "must never reach them")
	owner.post(open, "this one may")

	if got := sock.nextMessage(); got.Content != "this one may" {
		t.Fatalf("first message = %q, want the one from the visible channel", got.Content)
	}
}

func TestRevokingViewChannelSilencesALiveSession(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Live Revoke")
	secret, _ := owner.textAndVoice(guild.ID)
	open := owner.newTextChannel(guild.ID, "open")
	member := owner.inviteMember(guild.ID)
	everyone := owner.everyone(guild.ID)

	sock := member.dial()
	sock.identify(member.token)

	owner.post(secret, "while still allowed")
	if got := sock.nextMessage(); got.Content != "while still allowed" {
		t.Fatalf("first message = %q, want the one sent before the denial", got.Content)
	}

	owner.denyView(secret, everyone.ID, "role")

	owner.post(secret, "must never reach them")
	owner.post(open, "this one may")

	if got := sock.nextMessage(); got.Content != "this one may" {
		t.Fatalf("message after the denial = %q; the same connection was not resilenced", got.Content)
	}
}

func TestGrantingViewChannelReachesALiveSession(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Live Grant")
	member := owner.inviteMember(guild.ID)
	secret, _ := owner.textAndVoice(guild.ID)
	everyone := owner.everyone(guild.ID)

	owner.denyView(secret, everyone.ID, "role")

	sock := member.dial()
	sock.identify(member.token)

	owner.post(secret, "must never reach them")

	owner.mustDo(http.MethodDelete,
		"/api/v1/channels/"+secret.String()+"/overwrites/"+everyone.ID.String(),
		http.StatusNoContent, nil, nil)

	owner.post(secret, "allowed again")

	if got := sock.nextMessage(); got.Content != "allowed again" {
		t.Fatalf("first message = %q, want only the one sent after the grant", got.Content)
	}
}

func TestGuildWideEventsAndNewChannelsSurviveTheFilter(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Still Delivered")
	member := owner.inviteMember(guild.ID)

	sock := member.dial()
	sock.identify(member.token)

	fresh := owner.newTextChannel(guild.ID, "brand-new")

	created := sock.readEvent(events.EventChannelCreate)
	var ch events.Channel
	decode(t, created.D, &ch)
	if ch.ID != fresh {
		t.Fatalf("CHANNEL_CREATE carried %s, want %s", ch.ID, fresh)
	}

	owner.post(fresh, "a channel nobody has restricted")
	if got := sock.nextMessage(); got.Content != "a channel nobody has restricted" {
		t.Fatalf("message in a new channel = %q; a channel is hidden unless denied", got.Content)
	}
}

func TestVoiceEventsRespectChannelVisibility(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Voice Visibility")
	watcher := owner.inviteMember(guild.ID)
	blocked := owner.inviteMember(guild.ID)
	blockedID, _ := memberIdentity(t, blocked)
	_, voice := owner.textAndVoice(guild.ID)

	owner.denyView(voice, blockedID, "member")

	watching := watcher.dial()
	watching.identify(watcher.token)
	deaf := blocked.dial()
	deaf.identify(blocked.token)

	ownerSock := owner.dial()
	ownerSock.identify(owner.token)
	ownerSock.write(events.Frame{
		Op: events.OpVoiceState,
		D:  mustJSON(t, events.VoiceStateRequest{ChannelID: &voice}),
	})

	state := watching.readEvent(events.EventVoiceStateUpdate)
	var update events.VoiceStateUpdate
	decode(t, state.D, &update)
	if update.ChannelID == nil || *update.ChannelID != voice {
		t.Fatalf("watcher saw channel %v, want %s", update.ChannelID, voice)
	}

	if !deaf.quietFor(2*time.Second, func(f events.Frame) bool {
		return f.T == events.EventVoiceStateUpdate
	}) {
		t.Error("a member denied ViewChannel still learns who is in the voice channel")
	}
}

func seesChannel(channels []events.Channel, id uuid.UUID) bool {
	for _, c := range channels {
		if c.ID == id {
			return true
		}
	}
	return false
}

func memberIdentity(t *testing.T, h *harness) (uuid.UUID, string) {
	t.Helper()
	var me events.User
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusOK, nil, &me)
	return me.ID, me.Username
}
