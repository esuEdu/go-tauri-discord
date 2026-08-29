//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) channelOrder(guildID uuid.UUID) []string {
	h.t.Helper()
	var channels []events.Channel
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/channels",
		http.StatusOK, nil, &channels)

	names := make([]string, 0, len(channels))
	for _, ch := range channels {
		names = append(names, ch.Name)
	}
	return names
}

func (h *harness) move(channelID uuid.UUID, to int, want int) {
	h.t.Helper()
	h.mustDo(http.MethodPatch, "/api/v1/channels/"+channelID.String()+"/position",
		want, map[string]any{"position": to}, nil)
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAChannelStaysWhereItWasPut(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Ordering")
	first, _ := owner.textAndVoice(guild.ID)
	second := owner.newTextChannel(guild.ID, "second")
	third := owner.newTextChannel(guild.ID, "third")
	_ = first

	before := owner.channelOrder(guild.ID)
	if len(before) < 4 {
		t.Fatalf("expected at least four channels, got %v", before)
	}

	owner.move(third, 0, http.StatusOK)

	after := owner.channelOrder(guild.ID)
	if after[0] != "third" {
		t.Fatalf("channel order = %v, want third first; a position that does not survive the "+
			"next listing is not persistence, it is a repaint", after)
	}
	if same(before, after) {
		t.Error("the order did not change at all")
	}

	owner.move(second, 99, http.StatusOK)
	tail := owner.channelOrder(guild.ID)
	if tail[len(tail)-1] != "second" {
		t.Errorf("moving past the end put the channel at %v, want it last", tail)
	}
}

func TestMovingAChannelNeedsManageChannels(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Not Yours")
	owner.newTextChannel(guild.ID, "second")
	third := owner.newTextChannel(guild.ID, "third")
	member := owner.inviteMember(guild.ID)

	member.move(third, 0, http.StatusForbidden)

	if order := owner.channelOrder(guild.ID); order[0] == "third" {
		t.Fatal("a member without ManageChannels rearranged the server for everybody")
	}
}

func TestReorderingIsAnnouncedToEverybody(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Announced")
	general, _ := owner.textAndVoice(guild.ID)
	owner.newTextChannel(guild.ID, "second")
	owner.newTextChannel(guild.ID, "third")
	member := owner.inviteMember(guild.ID)

	sock := member.dial()
	sock.identify(member.token)

	owner.move(general, 99, http.StatusOK)

	moved := awaitChannelUpdate(t, sock, general)
	if int(moved.Position) != len(owner.channelOrder(guild.ID))-1 {
		t.Errorf("the moved channel was announced at position %d, want last; without the new "+
			"position everybody keeps the old order until they reload", moved.Position)
	}
}

func awaitChannelUpdate(t *testing.T, s *socket, channelID uuid.UUID) events.Channel {
	t.Helper()
	for range 10 {
		var updated events.Channel
		decode(t, s.readEvent(events.EventChannelUpdate).D, &updated)
		if updated.ID == channelID {
			return updated
		}
	}
	t.Fatalf("no channel update about %s arrived", channelID)
	return events.Channel{}
}
