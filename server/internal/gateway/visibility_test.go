package gateway

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func frameFor(t *testing.T, kind events.EventType, payload any) []byte {
	t.Helper()
	frame, err := events.NewDispatch(kind, payload)
	if err != nil {
		t.Fatalf("dispatch %s: %v", kind, err)
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal %s: %v", kind, err)
	}
	return raw
}

func TestScopedChannelRecognisesEveryChannelBoundEvent(t *testing.T) {
	channelID := uuid.New()

	bound := []struct {
		kind    events.EventType
		payload any
	}{
		{events.EventMessageCreate, events.Message{ChannelID: channelID}},
		{events.EventMessageUpdate, events.Message{ChannelID: channelID}},
		{events.EventMessageDelete, events.MessageDelete{ChannelID: channelID}},
		{events.EventTypingStart, events.TypingStart{ChannelID: channelID}},
		{events.EventVoiceStateUpdate, events.VoiceStateUpdate{ChannelID: &channelID}},
		{events.EventVoiceScreenUpdate, events.VoiceScreenUpdate{ChannelID: channelID}},
	}

	updates := frameFor(t, events.EventChannelUpdate, events.Channel{ID: channelID})
	got, scoped := scopedChannel(updates)
	if !scoped || got != channelID {
		t.Error("CHANNEL_UPDATE is not treated as channel-bound, so renaming or moving a hidden " +
			"channel tells everybody it exists")
	}

	for _, tt := range bound {
		got, scoped := scopedChannel(frameFor(t, tt.kind, tt.payload))
		if !scoped {
			t.Errorf("%s is not treated as channel-bound; it would bypass the filter", tt.kind)
			continue
		}
		if got != channelID {
			t.Errorf("%s carried channel %s, want %s", tt.kind, got, channelID)
		}
	}
}

func TestGuildWideEventsAreNotFiltered(t *testing.T) {
	wide := []struct {
		kind    events.EventType
		payload any
	}{
		{events.EventPresenceUpdate, events.PresenceUpdate{UserID: uuid.New(), Status: "online"}},
		{events.EventGuildCreate, events.Guild{ID: uuid.New(), Name: "Somewhere"}},
		{events.EventChannelCreate, events.Channel{ID: uuid.New(), GuildID: uuid.New()}},
	}

	for _, tt := range wide {
		if _, scoped := scopedChannel(frameFor(t, tt.kind, tt.payload)); scoped {
			t.Errorf("%s was filtered as channel-bound; it must reach every member", tt.kind)
		}
	}
}

func TestLeavingVoiceIsNotFiltered(t *testing.T) {
	raw := frameFor(t, events.EventVoiceStateUpdate, events.VoiceStateUpdate{ChannelID: nil})
	if _, scoped := scopedChannel(raw); scoped {
		t.Error("a null channel means the member left voice; there is nothing to hide")
	}
}

func TestUnreadableFramesAreNotFiltered(t *testing.T) {
	if _, scoped := scopedChannel([]byte("{not json")); scoped {
		t.Error("a frame that cannot be parsed must not be claimed as channel-bound")
	}
}

func TestHideInGuildReplacesOnlyThatGuild(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()

	s := &session{}
	s.hideInGuild(first, []uuid.UUID{a, b})
	s.hideInGuild(second, []uuid.UUID{c})

	for _, id := range []uuid.UUID{a, b, c} {
		if s.canSee(id) {
			t.Errorf("channel %s should be hidden", id)
		}
	}

	s.hideInGuild(first, []uuid.UUID{b})

	if s.canSee(b) {
		t.Error("a channel still denied must stay hidden")
	}
	if !s.canSee(a) {
		t.Error("a channel no longer denied must become visible again")
	}
	if s.canSee(c) {
		t.Error("refreshing one guild must not disturb another")
	}
}

func TestAnUnknownChannelIsVisible(t *testing.T) {
	s := &session{}
	if !s.canSee(uuid.New()) {
		t.Error("a channel nobody has denied must be visible, or new channels go dark")
	}
}
