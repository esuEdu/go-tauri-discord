package message

import (
	"context"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const PreviewLen = 120

func (s *Service) parentOf(ctx context.Context, replyTo *uuid.UUID, channelID uuid.UUID) (*uuid.UUID, error) {
	if replyTo == nil {
		return nil, nil
	}

	parent, err := s.repo.GetMessage(ctx, *replyTo)
	if err != nil {
		if db.IsNoRows(err) {
			return nil, domain.Invalid("the message being replied to no longer exists")
		}
		return nil, domain.Internal(err)
	}
	if parent.ChannelID != channelID {
		return nil, domain.Invalid("a reply must stay in the channel it answers")
	}
	return &parent.ID, nil
}

func (s *Service) previewOf(row dbgen.ListMessagePreviewsRow) events.ReplyPreview {
	preview := events.ReplyPreview{
		MessageID:      row.ID,
		Content:        row.Content,
		Truncated:      row.Truncated,
		HasAttachments: row.HasAttachments,
		Deleted:        row.Deleted,
	}
	if row.AuthorID != nil && row.AuthorUsername != nil {
		preview.Author = &events.User{
			ID:            *row.AuthorID,
			Username:      *row.AuthorUsername,
			Discriminator: value(row.AuthorDiscriminator),
			AvatarKey:     row.AuthorAvatarKey,
		}
	}
	return preview
}

func (s *Service) attachReplies(ctx context.Context, msgs []events.Message, parents []uuid.UUID) error {
	if len(parents) == 0 {
		return nil
	}

	rows, err := s.repo.ListMessagePreviews(ctx, dbgen.ListMessagePreviewsParams{
		PreviewLen: PreviewLen, MessageIds: parents,
	})
	if err != nil {
		return domain.Internal(err)
	}

	byID := make(map[uuid.UUID]events.ReplyPreview, len(rows))
	for _, row := range rows {
		byID[row.ID] = s.previewOf(row)
	}
	for i := range msgs {
		if msgs[i].ReplyTo == nil {
			continue
		}
		if got, ok := byID[msgs[i].ReplyTo.MessageID]; ok {
			preview := got
			msgs[i].ReplyTo = &preview
			continue
		}
		msgs[i].ReplyTo = nil
	}
	return nil
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func replyStub(parentID *uuid.UUID) *events.ReplyPreview {
	if parentID == nil {
		return nil
	}
	return &events.ReplyPreview{MessageID: *parentID}
}

func replyTargets(msgs []events.Message) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(msgs))
	ids := make([]uuid.UUID, 0, len(msgs))
	for _, m := range msgs {
		if m.ReplyTo == nil {
			continue
		}
		if _, done := seen[m.ReplyTo.MessageID]; done {
			continue
		}
		seen[m.ReplyTo.MessageID] = struct{}{}
		ids = append(ids, m.ReplyTo.MessageID)
	}
	return ids
}
