//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) renameGuild(guildID uuid.UUID, body map[string]any, want int) events.Guild {
	h.t.Helper()
	var g events.Guild
	if want == http.StatusOK {
		h.mustDo(http.MethodPatch, "/api/v1/guilds/"+guildID.String(), want, body, &g)
		return g
	}
	h.mustDo(http.MethodPatch, "/api/v1/guilds/"+guildID.String(), want, body, nil)
	return g
}

func (h *harness) guildNamed(guildID uuid.UUID) string {
	h.t.Helper()
	var guilds []events.Guild
	h.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &guilds)
	for _, g := range guilds {
		if g.ID == guildID {
			return g.Name
		}
	}
	h.t.Fatalf("no guild %s in the list", guildID)
	return ""
}

func TestAGuildCanBeRenamed(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Old Name")

	renamed := me.renameGuild(guild.ID, map[string]any{"name": "New Name"}, http.StatusOK)
	if renamed.Name != "New Name" {
		t.Errorf("name = %q, want New Name", renamed.Name)
	}
	if got := me.guildNamed(guild.ID); got != "New Name" {
		t.Errorf("re-read name = %q, the rename did not persist", got)
	}
}

func TestRenamingKeepsTheOwnerAndIcon(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Keeps Things")

	renamed := me.renameGuild(guild.ID, map[string]any{"name": "Still Mine"}, http.StatusOK)
	if renamed.OwnerID != guild.OwnerID {
		t.Errorf("owner = %s, want %s", renamed.OwnerID, guild.OwnerID)
	}
	if renamed.ID != guild.ID {
		t.Errorf("id = %s, want %s", renamed.ID, guild.ID)
	}
}

func TestABlankGuildNameIsRefused(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Unchanged")

	me.renameGuild(guild.ID, map[string]any{"name": "   "}, http.StatusBadRequest)

	if got := me.guildNamed(guild.ID); got != "Unchanged" {
		t.Errorf("name = %q, a refused rename should change nothing", got)
	}
}

func TestRenamingAGuildNeedsManageGuild(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Owned")
	member := owner.inviteMember(guild.ID)

	member.renameGuild(guild.ID, map[string]any{"name": "Mine Now"}, http.StatusForbidden)

	if got := owner.guildNamed(guild.ID); got != "Owned" {
		t.Errorf("name = %q, an ordinary member renamed the server", got)
	}
}

func TestEveryMemberHearsARename(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Before")
	member := owner.inviteMember(guild.ID)

	sock := member.dial()
	sock.identify(member.token)

	owner.renameGuild(guild.ID, map[string]any{"name": "After"}, http.StatusOK)

	var heard events.Guild
	decode(t, sock.readEvent(events.EventGuildUpdate).D, &heard)
	if heard.Name != "After" {
		t.Errorf("event carried name %q, want After", heard.Name)
	}
	if heard.ID != guild.ID {
		t.Errorf("event carried guild %s, want %s", heard.ID, guild.ID)
	}
}
