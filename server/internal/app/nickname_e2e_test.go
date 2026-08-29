//go:build e2e

package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) setNickname(guildID, userID uuid.UUID, body map[string]any, want int) events.Member {
	h.t.Helper()
	path := "/api/v1/guilds/" + guildID.String() + "/members/" + userID.String()
	var member events.Member
	if want == http.StatusOK {
		h.mustDo(http.MethodPatch, path, want, body, &member)
		return member
	}
	h.mustDo(http.MethodPatch, path, want, body, nil)
	return member
}

func (h *harness) nicknameOf(guildID, userID uuid.UUID) *string {
	h.t.Helper()
	var members []struct {
		UserID   uuid.UUID `json:"user_id"`
		Nickname *string   `json:"nickname"`
	}
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/members",
		http.StatusOK, nil, &members)
	for _, m := range members {
		if m.UserID == userID {
			return m.Nickname
		}
	}
	h.t.Fatalf("no member %s", userID)
	return nil
}

func TestYouCanNameYourself(t *testing.T) {
	me := newHarness(t)
	myID, _ := me.registerUser()
	guild := me.createGuild("Naming")

	member := me.setNickname(guild.ID, myID, map[string]any{"nickname": "Marta the Brave"},
		http.StatusOK)
	if member.Nickname == nil || *member.Nickname != "Marta the Brave" {
		t.Errorf("nickname = %v, want Marta the Brave", member.Nickname)
	}
	if got := me.nicknameOf(guild.ID, myID); got == nil || *got != "Marta the Brave" {
		t.Errorf("re-read nickname = %v, it did not persist", got)
	}
}

func TestClearingANicknameGoesBackToTheUsername(t *testing.T) {
	me := newHarness(t)
	myID, _ := me.registerUser()
	guild := me.createGuild("Undoing")

	me.setNickname(guild.ID, myID, map[string]any{"nickname": "Temporary"}, http.StatusOK)

	member := me.setNickname(guild.ID, myID, map[string]any{"nickname": nil}, http.StatusOK)
	if member.Nickname != nil {
		t.Errorf("nickname = %q, sending null should clear it", *member.Nickname)
	}
	if got := me.nicknameOf(guild.ID, myID); got != nil {
		t.Errorf("re-read nickname = %q, the clear did not persist", *got)
	}
	if member.User.Username == "" {
		t.Error("the reply carries no username, so nothing can fall back to it")
	}
}

func TestABlankNicknameClearsRatherThanStoringSpaces(t *testing.T) {
	me := newHarness(t)
	myID, _ := me.registerUser()
	guild := me.createGuild("Whitespace")

	me.setNickname(guild.ID, myID, map[string]any{"nickname": "Something"}, http.StatusOK)

	member := me.setNickname(guild.ID, myID, map[string]any{"nickname": "   "}, http.StatusOK)
	if member.Nickname != nil {
		t.Errorf("nickname = %q, blank should clear rather than store spaces", *member.Nickname)
	}
}

func TestATooLongNicknameIsRefused(t *testing.T) {
	me := newHarness(t)
	myID, _ := me.registerUser()
	guild := me.createGuild("Too Long")

	me.setNickname(guild.ID, myID,
		map[string]any{"nickname": strings.Repeat("x", 33)}, http.StatusBadRequest)

	if got := me.nicknameOf(guild.ID, myID); got != nil {
		t.Errorf("nickname = %q, a refused name should store nothing", *got)
	}
}

func TestNamingSomebodyElseNeedsManageGuild(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Guarded Names")
	member := owner.inviteMember(guild.ID)
	memberID := member.whoAmI()

	member.setNickname(guild.ID, ownerID, map[string]any{"nickname": "Not Yours"},
		http.StatusForbidden)

	renamed := owner.setNickname(guild.ID, memberID, map[string]any{"nickname": "Helper"},
		http.StatusOK)
	if renamed.Nickname == nil || *renamed.Nickname != "Helper" {
		t.Errorf("nickname = %v, the owner should be able to name a member", renamed.Nickname)
	}
}

func TestANicknameIsAnnouncedToTheGuild(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Announcing")
	member := owner.inviteMember(guild.ID)

	sock := member.dial()
	sock.identify(member.token)

	owner.setNickname(guild.ID, ownerID, map[string]any{"nickname": "The Boss"}, http.StatusOK)

	var heard events.Member
	decode(t, sock.readEvent(events.EventGuildMemberUpdate).D, &heard)
	if heard.GuildID != guild.ID {
		t.Fatalf("event carried guild %s, want %s", heard.GuildID, guild.ID)
	}
	if heard.Nickname == nil || *heard.Nickname != "The Boss" {
		t.Errorf("event carried nickname %v, want The Boss", heard.Nickname)
	}
	if heard.User.ID != ownerID {
		t.Errorf("event carried user %s, want %s", heard.User.ID, ownerID)
	}
}

func TestANicknameSurvivesReconnecting(t *testing.T) {
	me := newHarness(t)
	myID, _ := me.registerUser()
	guild := me.createGuild("Reconnecting")

	me.setNickname(guild.ID, myID, map[string]any{"nickname": "Persistent"}, http.StatusOK)

	sock := me.dial()
	ready := sock.identify(me.token)

	for _, m := range ready.Members {
		if m.User.ID == myID && m.GuildID == guild.ID {
			if m.Nickname == nil || *m.Nickname != "Persistent" {
				t.Errorf("READY carried nickname %v, want Persistent", m.Nickname)
			}
			return
		}
	}
	t.Error("READY did not mention us at all")
}
