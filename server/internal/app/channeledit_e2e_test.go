//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) editChannel(channelID uuid.UUID, body map[string]any, want int) events.Channel {
	h.t.Helper()
	var ch events.Channel
	if want == http.StatusOK {
		h.mustDo(http.MethodPatch, "/api/v1/channels/"+channelID.String(), want, body, &ch)
		return ch
	}
	h.mustDo(http.MethodPatch, "/api/v1/channels/"+channelID.String(), want, body, nil)
	return ch
}

func (h *harness) removeChannel(channelID uuid.UUID, want int) {
	h.t.Helper()
	h.mustDo(http.MethodDelete, "/api/v1/channels/"+channelID.String(), want, nil, nil)
}

func (h *harness) hasChannel(guildID uuid.UUID, name string) bool {
	h.t.Helper()
	for _, ch := range h.channelsOf(guildID) {
		if ch.Name == name {
			return true
		}
	}
	return false
}

func TestAChannelCanBeRenamedAndGivenATopic(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Editing")

	made := me.newChannel(guild.ID, map[string]any{
		"name": "old-name", "kind": "text", "position": 0,
	})

	renamed := me.editChannel(made.ID, map[string]any{"name": "new-name"}, http.StatusOK)
	if renamed.Name != "new-name" {
		t.Errorf("name = %q, want new-name", renamed.Name)
	}
	if !me.hasChannel(guild.ID, "new-name") {
		t.Error("the rename did not survive a re-read")
	}

	withTopic := me.editChannel(made.ID, map[string]any{"topic": "what we are up to"}, http.StatusOK)
	if withTopic.Topic == nil || *withTopic.Topic != "what we are up to" {
		t.Errorf("topic = %v, want the text we sent", withTopic.Topic)
	}
	if withTopic.Name != "new-name" {
		t.Errorf("name = %q, setting only the topic should leave the name alone", withTopic.Name)
	}
}

func TestSendingNullClearsATopicWhileLeavingItOutKeepsIt(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Topics")

	made := me.newChannel(guild.ID, map[string]any{
		"name": "notes", "kind": "text", "position": 0,
	})
	me.editChannel(made.ID, map[string]any{"topic": "the first topic"}, http.StatusOK)

	kept := me.editChannel(made.ID, map[string]any{"name": "notes"}, http.StatusOK)
	if kept.Topic == nil || *kept.Topic != "the first topic" {
		t.Errorf("topic = %v, an absent key should leave the topic alone", kept.Topic)
	}

	cleared := me.editChannel(made.ID, map[string]any{"topic": nil}, http.StatusOK)
	if cleared.Topic != nil {
		t.Errorf("topic = %v, an explicit null should clear it", *cleared.Topic)
	}
}

func TestAnEmptyNameIsRefused(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Naming")

	made := me.newChannel(guild.ID, map[string]any{
		"name": "keep-me", "kind": "text", "position": 0,
	})

	me.editChannel(made.ID, map[string]any{"name": "   "}, http.StatusBadRequest)

	if !me.hasChannel(guild.ID, "keep-me") {
		t.Error("a refused rename should leave the channel as it was")
	}
}

func TestADeletedChannelIsGoneAndItsChildrenComeLoose(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	guild := me.createGuild("Deleting")

	category := me.newChannel(guild.ID, map[string]any{
		"name": "Projects", "kind": "category", "position": 0,
	})
	inside := me.newChannel(guild.ID, map[string]any{
		"name": "scratch", "kind": "text", "position": 1,
		"parent_id": category.ID.String(),
	})

	if got := me.parentOf(guild.ID, "scratch"); got == nil || *got != category.ID {
		t.Fatalf("parent = %v, want the category before we delete it", got)
	}

	me.removeChannel(category.ID, http.StatusNoContent)

	if me.hasChannel(guild.ID, "Projects") {
		t.Error("the category is still listed after being deleted")
	}
	if !me.hasChannel(guild.ID, "scratch") {
		t.Fatal("deleting a category took its channels with it")
	}
	if got := me.parentOf(guild.ID, "scratch"); got != nil {
		t.Errorf("parent = %v, a child of a deleted category should come loose", *got)
	}

	me.removeChannel(inside.ID, http.StatusNoContent)
	if me.hasChannel(guild.ID, "scratch") {
		t.Error("the channel is still listed after being deleted")
	}
}

func TestEditingAChannelNeedsManageChannels(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Guarded")
	made := owner.newChannel(guild.ID, map[string]any{
		"name": "locked", "kind": "text", "position": 0,
	})
	invite := owner.createInvite(guild.ID, map[string]any{})

	member := newHarness(t)
	member.registerUser()
	member.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	member.editChannel(made.ID, map[string]any{"name": "mine-now"}, http.StatusForbidden)
	member.removeChannel(made.ID, http.StatusForbidden)

	if !owner.hasChannel(guild.ID, "locked") {
		t.Error("an ordinary member managed to delete a channel")
	}
}
