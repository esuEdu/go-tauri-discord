package pubsub

import (
	"testing"

	"github.com/google/uuid"
)

func TestControlGuildRoundTrips(t *testing.T) {
	id := uuid.New()

	got, ok := ControlGuild(TopicGuildControl(id))
	if !ok {
		t.Fatal("a control topic was not recognised as one")
	}
	if got != id {
		t.Errorf("guild = %s, want %s", got, id)
	}
}

func TestOnlyControlTopicsAreControlTopics(t *testing.T) {
	id := uuid.New()

	for _, topic := range []string{
		TopicGuild(id),
		TopicUser(id),
		"guildctl:not-a-uuid",
		"",
	} {
		if _, ok := ControlGuild(topic); ok {
			t.Errorf("%q was mistaken for a control topic; its frames would never reach a client", topic)
		}
	}
}
