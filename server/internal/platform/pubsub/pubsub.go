package pubsub

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type Broker interface {
	Publish(ctx context.Context, topic string, payload []byte) error

	Subscribe(topic string) (<-chan []byte, func())
	Close() error
}

func TopicGuild(id uuid.UUID) string { return "guild:" + id.String() }

func TopicUser(id uuid.UUID) string { return "user:" + id.String() }

const controlPrefix = "guildctl:"

func TopicGuildControl(id uuid.UUID) string { return controlPrefix + id.String() }

func ControlGuild(topic string) (uuid.UUID, bool) {
	rest, found := strings.CutPrefix(topic, controlPrefix)
	if !found {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
