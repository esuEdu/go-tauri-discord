package bus

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/platform/pubsub"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Publisher struct {
	broker pubsub.Broker
}

func NewPublisher(broker pubsub.Broker) *Publisher { return &Publisher{broker: broker} }

func (p *Publisher) ToGuild(ctx context.Context, guildID uuid.UUID, t events.EventType, d any) {
	p.publish(ctx, pubsub.TopicGuild(guildID), t, d)
}

func (p *Publisher) ToUser(ctx context.Context, userID uuid.UUID, t events.EventType, d any) {
	p.publish(ctx, pubsub.TopicUser(userID), t, d)
}

func (p *Publisher) publish(ctx context.Context, topic string, t events.EventType, d any) {
	frame, err := events.NewDispatch(t, d)
	if err != nil {
		slog.ErrorContext(ctx, "marshal event payload", "event", t, "error", err)
		return
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		slog.ErrorContext(ctx, "marshal event frame", "event", t, "error", err)
		return
	}
	if err := p.broker.Publish(ctx, topic, raw); err != nil {
		slog.ErrorContext(ctx, "publish event", "event", t, "topic", topic, "error", err)
	}
}
