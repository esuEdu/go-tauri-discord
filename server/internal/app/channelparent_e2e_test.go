//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) channelsOf(guildID uuid.UUID) []events.Channel {
	h.t.Helper()
	var channels []events.Channel
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/channels",
		http.StatusOK, nil, &channels)
	return channels
}

func (h *harness) parentOf(guildID uuid.UUID, name string) *uuid.UUID {
	h.t.Helper()
	for _, ch := range h.channelsOf(guildID) {
		if ch.Name == name {
			return ch.ParentID
		}
	}
	h.t.Fatalf("no channel called %q", name)
	return nil
}

func (h *harness) newChannel(guildID uuid.UUID, body map[string]any) events.Channel {
	h.t.Helper()
	var ch events.Channel
	h.mustDo(http.MethodPost, "/api/v1/guilds/"+guildID.String()+"/channels",
		http.StatusCreated, body, &ch)
	return ch
}

func (h *harness) moveInto(channelID uuid.UUID, body map[string]any, want int) {
	h.t.Helper()
	h.mustDo(http.MethodPatch, "/api/v1/channels/"+channelID.String()+"/position",
		want, body, nil)
}

func (h *harness) named(guildID uuid.UUID, name string) events.Channel {
	h.t.Helper()
	for _, ch := range h.channelsOf(guildID) {
		if ch.Name == name {
			return ch
		}
	}
	h.t.Fatalf("no channel called %q", name)
	return events.Channel{}
}

func TestAChannelCanBeDraggedIntoACategoryAndBackOut(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Sorting")

	category := me.newChannel(guild.ID, map[string]any{
		"name": "Projects", "kind": "category", "position": 0,
	})
	loose := me.newChannel(guild.ID, map[string]any{
		"name": "loose", "kind": "text", "position": 1,
	})

	if me.parentOf(guild.ID, "loose") != nil {
		t.Fatal("a fresh channel should start outside every category")
	}

	me.moveInto(loose.ID, map[string]any{
		"position": 0, "parent_id": category.ID.String(),
	}, http.StatusOK)

	got := me.parentOf(guild.ID, "loose")
	if got == nil || *got != category.ID {
		t.Errorf("parent = %v, want the category %s", got, category.ID)
	}

	me.moveInto(loose.ID, map[string]any{"position": 0, "parent_id": nil}, http.StatusOK)

	if me.parentOf(guild.ID, "loose") != nil {
		t.Error("a channel dragged out of a category is still inside it")
	}
}

func TestMovingWithoutSayingAParentLeavesTheChannelWhereItLives(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Staying")

	category := me.newChannel(guild.ID, map[string]any{
		"name": "Projects", "kind": "category", "position": 0,
	})
	inside := me.newChannel(guild.ID, map[string]any{
		"name": "inside", "kind": "text", "position": 1,
	})
	me.moveInto(inside.ID, map[string]any{
		"position": 0, "parent_id": category.ID.String(),
	}, http.StatusOK)

	me.moveInto(inside.ID, map[string]any{"position": 0}, http.StatusOK)

	got := me.parentOf(guild.ID, "inside")
	if got == nil || *got != category.ID {
		t.Errorf("parent = %v; a plain reorder emptied the category", got)
	}
}

func TestACategoryCannotBePutInsideAnother(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Nesting")

	outer := me.newChannel(guild.ID, map[string]any{
		"name": "Outer", "kind": "category", "position": 0,
	})
	inner := me.newChannel(guild.ID, map[string]any{
		"name": "Inner", "kind": "category", "position": 1,
	})

	me.moveInto(inner.ID, map[string]any{
		"position": 0, "parent_id": outer.ID.String(),
	}, http.StatusBadRequest)

	if me.parentOf(guild.ID, "Inner") != nil {
		t.Error("a category was nested inside another")
	}
}

func TestAChannelCannotBePutInsideAPlainChannel(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Wrong parent")

	host := me.newChannel(guild.ID, map[string]any{
		"name": "host", "kind": "text", "position": 0,
	})
	guest := me.newChannel(guild.ID, map[string]any{
		"name": "guest", "kind": "text", "position": 1,
	})

	me.moveInto(guest.ID, map[string]any{
		"position": 0, "parent_id": host.ID.String(),
	}, http.StatusBadRequest)
	me.moveInto(guest.ID, map[string]any{
		"position": 0, "parent_id": uuid.New().String(),
	}, http.StatusNotFound)

	if me.parentOf(guild.ID, "guest") != nil {
		t.Error("a channel ended up inside something that is not a category")
	}
}

func TestOrderIsCountedAmongTheChannelsSharingAParent(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Ordering")

	category := me.newChannel(guild.ID, map[string]any{
		"name": "Projects", "kind": "category", "position": 0,
	})
	for _, name := range []string{"alpha", "beta", "gamma"} {
		ch := me.newChannel(guild.ID, map[string]any{
			"name": name, "kind": "text", "position": 9,
		})
		me.moveInto(ch.ID, map[string]any{
			"position": 9, "parent_id": category.ID.String(),
		}, http.StatusOK)
	}

	gamma := me.named(guild.ID, "gamma")
	me.moveInto(gamma.ID, map[string]any{"position": 0}, http.StatusOK)

	inside := []string{}
	for _, ch := range me.channelsOf(guild.ID) {
		if ch.ParentID != nil && *ch.ParentID == category.ID {
			inside = append(inside, ch.Name)
		}
	}
	if !same(inside, []string{"gamma", "alpha", "beta"}) {
		t.Errorf("inside the category = %v, want [gamma alpha beta]", inside)
	}
}
