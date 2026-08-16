package pubsub

import (
	"context"

	"github.com/google/uuid"
)

type Broker interface {
	Publish(ctx context.Context, topic string, payload []byte) error

	Subscribe(topic string) (<-chan []byte, func())
	Close() error
}

func TopicGuild(id uuid.UUID) string { return "guild:" + id.String() }

func TopicUser(id uuid.UUID) string { return "user:" + id.String() }
