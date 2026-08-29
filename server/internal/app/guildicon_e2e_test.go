//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func TestChangingTheIconTellsEveryMember(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Picture Book")
	member := owner.inviteMember(guild.ID)

	sock := member.dial()
	sock.identify(member.token)

	if status, _ := owner.putImage("/api/v1/guilds/"+guild.ID.String()+"/icon",
		aPNG(t, 64, 64), "image/png"); status != http.StatusOK {
		t.Fatalf("PUT icon = %d, want 200", status)
	}

	var heard events.Guild
	decode(t, sock.readEvent(events.EventGuildUpdate).D, &heard)
	if heard.ID != guild.ID {
		t.Fatalf("event carried guild %s, want %s", heard.ID, guild.ID)
	}
	if heard.IconKey == nil || *heard.IconKey == "" {
		t.Error("the event carried no icon key, so nobody else can fetch the new picture")
	}
	if heard.Name != "Picture Book" {
		t.Errorf("name = %q, an icon change should not disturb the name", heard.Name)
	}
}

func TestClearingTheIconTellsEveryMember(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Plain Again")
	member := owner.inviteMember(guild.ID)

	owner.putImage("/api/v1/guilds/"+guild.ID.String()+"/icon", aPNG(t, 64, 64), "image/png")

	sock := member.dial()
	sock.identify(member.token)

	owner.mustDo(http.MethodDelete, "/api/v1/guilds/"+guild.ID.String()+"/icon",
		http.StatusNoContent, nil, nil)

	var heard events.Guild
	decode(t, sock.readEvent(events.EventGuildUpdate).D, &heard)
	if heard.IconKey != nil {
		t.Errorf("icon_key = %q, clearing should announce an empty icon", *heard.IconKey)
	}
}

func TestAnOrdinaryMemberCannotChangeTheIcon(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Guarded Picture")
	member := owner.inviteMember(guild.ID)

	status, _ := member.putImage("/api/v1/guilds/"+guild.ID.String()+"/icon",
		aPNG(t, 64, 64), "image/png")
	if status != http.StatusForbidden {
		t.Errorf("PUT icon as an ordinary member = %d, want 403", status)
	}
}
