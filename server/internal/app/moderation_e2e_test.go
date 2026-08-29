//go:build e2e

package app_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type banView struct {
	GuildID   uuid.UUID  `json:"guild_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Username  string     `json:"username"`
	BannedBy  *uuid.UUID `json:"banned_by"`
	Reason    *string    `json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
}

func memberPath(guildID, userID uuid.UUID) string {
	return "/api/v1/guilds/" + guildID.String() + "/members/" + userID.String()
}

func banPath(guildID, userID uuid.UUID) string {
	return "/api/v1/guilds/" + guildID.String() + "/bans/" + userID.String()
}

func (h *harness) bans(guildID uuid.UUID) []banView {
	h.t.Helper()
	var out []banView
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/bans", http.StatusOK, nil, &out)
	return out
}

func (h *harness) inGuild(guildID uuid.UUID) bool {
	h.t.Helper()
	var mine []events.Guild
	h.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &mine)
	for _, g := range mine {
		if g.ID == guildID {
			return true
		}
	}
	return false
}

func TestKickTakesSomebodyOutAndCutsThemOff(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Kick")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	messages := "/api/v1/channels/" + text.String() + "/messages"
	member.mustDo(http.MethodPost, messages, http.StatusCreated,
		map[string]string{"content": "while I am still here"}, nil)

	theirs := member.dial()
	theirs.identify(member.token)
	ours := owner.dial()
	ours.identify(owner.token)

	owner.mustDo(http.MethodDelete, memberPath(guild.ID, memberID), http.StatusNoContent, nil, nil)

	var departure events.GuildRemoval
	decode(t, theirs.readEvent(events.EventGuildRemove).D, &departure)
	if departure.GuildID != guild.ID || departure.UserID != memberID {
		t.Errorf("GUILD_REMOVE = %+v, want guild %s and user %s", departure, guild.ID, memberID)
	}
	if departure.Banned {
		t.Error("a kick is not a ban, and the client is told so")
	}

	var seen events.GuildRemoval
	decode(t, ours.readEvent(events.EventGuildMemberRemove).D, &seen)
	if seen.UserID != memberID {
		t.Errorf("the people still here were told %s left, want %s", seen.UserID, memberID)
	}

	if member.inGuild(guild.ID) {
		t.Error("the kicked member still lists the guild")
	}
	member.mustDo(http.MethodPost, messages, http.StatusNotFound,
		map[string]string{"content": "still talking"}, nil)

	owner.mustDo(http.MethodPost, messages, http.StatusCreated,
		map[string]string{"content": "after they left"}, nil)
	quiet := theirs.quietFor(500*time.Millisecond, func(f events.Frame) bool {
		return f.T == events.EventMessageCreate
	})
	if !quiet {
		t.Error("the kicked member's socket is still receiving the guild's messages")
	}

	history := owner.history(text, "")
	kept := false
	for _, m := range history {
		if m.Content == "while I am still here" {
			kept = true
			if m.Author.ID != memberID {
				t.Errorf("author = %s, want the kicked member %s: a kick deletes nothing", m.Author.ID, memberID)
			}
		}
	}
	if !kept {
		t.Error("what the kicked member wrote is gone; a kick must not delete messages")
	}

	if len(owner.bans(guild.ID)) != 0 {
		t.Error("a kick recorded a ban")
	}
}

func TestAKickedMemberComesBackWithoutTheirOldRoles(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Second Chances")
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	role := owner.createRole(guild.ID, map[string]any{
		"name":        "Staff",
		"permissions": perm(domain.PermManageChannels),
		"position":    3,
	})
	owner.mustDo(http.MethodPut, memberRolesPath(guild.ID, memberID, role.ID),
		http.StatusNoContent, nil, nil)

	channels := "/api/v1/guilds/" + guild.ID.String() + "/channels"
	newChannel := map[string]any{"name": "staff-only", "kind": "text"}
	member.mustDo(http.MethodPost, channels, http.StatusCreated, newChannel, nil)

	owner.mustDo(http.MethodDelete, memberPath(guild.ID, memberID), http.StatusNoContent, nil, nil)

	invite := owner.createInvite(guild.ID, map[string]any{})
	member.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	if !member.inGuild(guild.ID) {
		t.Fatal("a kick must not stop them coming back with a new invite")
	}
	member.mustDo(http.MethodPost, channels, http.StatusForbidden, newChannel, nil)
}

func TestBanKeepsSomebodyOutUntilItIsLifted(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Bans")
	member := owner.inviteMember(guild.ID)
	memberID, name := memberIdentity(t, member)

	var recorded banView
	owner.mustDo(http.MethodPut, banPath(guild.ID, memberID), http.StatusCreated,
		map[string]any{"reason": "  spent the evening shouting  "}, &recorded)
	if recorded.UserID != memberID || recorded.Username != name {
		t.Errorf("ban = %+v, want %s (%s)", recorded, memberID, name)
	}
	if recorded.BannedBy == nil || *recorded.BannedBy != ownerID {
		t.Errorf("banned_by = %v, want %s", recorded.BannedBy, ownerID)
	}
	if recorded.Reason == nil || *recorded.Reason != "spent the evening shouting" {
		t.Errorf("reason = %v, want it trimmed and kept", recorded.Reason)
	}

	if member.inGuild(guild.ID) {
		t.Error("a ban did not remove the membership")
	}

	invite := owner.createInvite(guild.ID, map[string]any{})
	member.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusForbidden, nil, nil)

	listed := owner.bans(guild.ID)
	if len(listed) != 1 || listed[0].UserID != memberID {
		t.Fatalf("bans = %+v, want exactly the one", listed)
	}

	owner.mustDo(http.MethodDelete, banPath(guild.ID, memberID), http.StatusNoContent, nil, nil)
	owner.mustDo(http.MethodDelete, banPath(guild.ID, memberID), http.StatusNotFound, nil, nil)
	if len(owner.bans(guild.ID)) != 0 {
		t.Error("the lifted ban is still listed")
	}

	member.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)
	if !member.inGuild(guild.ID) {
		t.Error("an unbanned person still cannot get back in")
	}
}

func TestSomebodyCanBeBannedBeforeTheyEverJoin(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Pre-emptive")
	stranger := owner.newUser()
	strangerID, _ := memberIdentity(t, stranger)

	owner.mustDo(http.MethodPut, banPath(guild.ID, strangerID), http.StatusCreated,
		map[string]any{"reason": nil}, nil)

	invite := owner.createInvite(guild.ID, map[string]any{})
	stranger.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusForbidden, nil, nil)

	owner.mustDo(http.MethodDelete, memberPath(guild.ID, strangerID), http.StatusNotFound, nil, nil)
}

func TestModerationRespectsTheHierarchy(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Hierarchy")

	mod := owner.inviteMember(guild.ID)
	modID, _ := memberIdentity(t, mod)
	peer := owner.inviteMember(guild.ID)
	peerID, _ := memberIdentity(t, peer)

	staff := owner.createRole(guild.ID, map[string]any{
		"name":        "Moderator",
		"permissions": perm(domain.PermKickMembers),
		"position":    5,
	})
	for _, id := range []uuid.UUID{modID, peerID} {
		owner.mustDo(http.MethodPut, memberRolesPath(guild.ID, id, staff.ID),
			http.StatusNoContent, nil, nil)
	}

	mod.mustDo(http.MethodDelete, memberPath(guild.ID, ownerID), http.StatusForbidden, nil, nil)
	mod.mustDo(http.MethodDelete, memberPath(guild.ID, peerID), http.StatusForbidden, nil, nil)
	mod.mustDo(http.MethodDelete, memberPath(guild.ID, modID), http.StatusBadRequest, nil, nil)
	mod.mustDo(http.MethodPut, banPath(guild.ID, peerID), http.StatusForbidden,
		map[string]any{"reason": nil}, nil)
	mod.mustDo(http.MethodGet, "/api/v1/guilds/"+guild.ID.String()+"/bans",
		http.StatusForbidden, nil, nil)

	owner.mustDo(http.MethodDelete, memberRolesPath(guild.ID, peerID, staff.ID),
		http.StatusNoContent, nil, nil)

	peer.mustDo(http.MethodDelete, memberPath(guild.ID, modID), http.StatusForbidden, nil, nil)
	mod.mustDo(http.MethodDelete, memberPath(guild.ID, peerID), http.StatusNoContent, nil, nil)
	if peer.inGuild(guild.ID) {
		t.Error("a moderator could not remove somebody below them")
	}

	owner.mustDo(http.MethodPut, banPath(guild.ID, modID), http.StatusCreated,
		map[string]any{"reason": nil}, nil)
	if mod.inGuild(guild.ID) {
		t.Error("the owner outranks everybody and could not remove a moderator")
	}
}

func TestBanningSomebodyInVoiceDisconnectsThem(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Voice Ban")
	_, voice := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	ours := owner.dial()
	ours.identify(owner.token)
	theirs := member.dial()
	theirs.identify(member.token)

	theirs.write(events.Frame{Op: events.OpVoiceState, D: mustJSON(t, events.VoiceStateRequest{
		ChannelID: &voice,
	})})

	var joined events.VoiceStateUpdate
	decode(t, ours.readUntil("them joining voice", func(f events.Frame) bool {
		if f.T != events.EventVoiceStateUpdate {
			return false
		}
		var state events.VoiceStateUpdate
		return json.Unmarshal(f.D, &state) == nil && state.UserID == memberID && state.ChannelID != nil
	}).D, &joined)

	owner.mustDo(http.MethodPut, banPath(guild.ID, memberID), http.StatusCreated,
		map[string]any{"reason": nil}, nil)

	var left events.VoiceStateUpdate
	decode(t, ours.readUntil("them leaving voice", func(f events.Frame) bool {
		if f.T != events.EventVoiceStateUpdate {
			return false
		}
		var state events.VoiceStateUpdate
		return json.Unmarshal(f.D, &state) == nil && state.UserID == memberID && state.ChannelID == nil
	}).D, &left)
	if left.UserID != memberID {
		t.Errorf("left = %+v, want the banned member out of voice", left)
	}
}

func TestLeavingTakesYouOutAndCutsYouOff(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Leave")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	messages := "/api/v1/channels/" + text.String() + "/messages"
	member.mustDo(http.MethodPost, messages, http.StatusCreated,
		map[string]string{"content": "before I go"}, nil)

	theirs := member.dial()
	theirs.identify(member.token)
	ours := owner.dial()
	ours.identify(owner.token)

	leave := "/api/v1/guilds/" + guild.ID.String() + "/members/@me"
	member.mustDo(http.MethodDelete, leave, http.StatusNoContent, nil, nil)

	var departure events.GuildRemoval
	decode(t, theirs.readEvent(events.EventGuildRemove).D, &departure)
	if departure.UserID != memberID || departure.Banned {
		t.Errorf("GUILD_REMOVE = %+v, want an unbanned %s", departure, memberID)
	}

	var seen events.GuildRemoval
	decode(t, ours.readEvent(events.EventGuildMemberRemove).D, &seen)
	if seen.UserID != memberID {
		t.Errorf("the people still here were told %s left, want %s", seen.UserID, memberID)
	}

	if member.inGuild(guild.ID) {
		t.Error("somebody who left still lists the guild")
	}
	member.mustDo(http.MethodPost, messages, http.StatusNotFound,
		map[string]string{"content": "still talking"}, nil)

	owner.mustDo(http.MethodPost, messages, http.StatusCreated,
		map[string]string{"content": "after they left"}, nil)
	quiet := theirs.quietFor(500*time.Millisecond, func(f events.Frame) bool {
		return f.T == events.EventMessageCreate
	})
	if !quiet {
		t.Error("the socket of somebody who left is still receiving the guild's messages")
	}

	kept := false
	for _, m := range owner.history(text, "") {
		if m.Content == "before I go" {
			kept = true
			if m.Author.ID != memberID {
				t.Errorf("author = %s, want %s: leaving deletes nothing", m.Author.ID, memberID)
			}
		}
	}
	if !kept {
		t.Error("what they wrote is gone; leaving must not delete messages")
	}
}

func TestTheOwnerCannotLeaveTheirOwnServer(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Mine")

	leave := "/api/v1/guilds/" + guild.ID.String() + "/members/@me"
	owner.mustDo(http.MethodDelete, leave, http.StatusForbidden, nil, nil)

	if !owner.inGuild(guild.ID) {
		t.Error("the owner left a server that would then belong to nobody")
	}
}

func TestLeavingAServerYouAreNotInIsNotFound(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Strangers")

	outsider := owner.newUser()
	leave := "/api/v1/guilds/" + guild.ID.String() + "/members/@me"
	outsider.mustDo(http.MethodDelete, leave, http.StatusNotFound, nil, nil)
}
