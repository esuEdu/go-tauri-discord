package message

import (
	"context"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const (
	MaxEmojiRunes = 8
	MaxEmojiKinds = 20
)

func validEmoji(emoji string) error {
	if emoji == "" {
		return domain.Invalid("a reaction needs an emoji")
	}
	if !utf8.ValidString(emoji) {
		return domain.Invalid("a reaction must be valid utf-8")
	}
	if utf8.RuneCountInString(emoji) > MaxEmojiRunes {
		return domain.Invalid("a reaction is at most %d characters", MaxEmojiRunes)
	}
	for _, r := range emoji {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || unicode.IsControl(r) {
			return domain.Invalid("a reaction must be an emoji, not text")
		}
	}
	return nil
}

func (s *Service) React(ctx context.Context, userID, messageID uuid.UUID, emoji string) error {
	if err := validEmoji(emoji); err != nil {
		return err
	}

	msg, channel, err := s.reactable(ctx, userID, messageID, domain.PermAddReactions)
	if err != nil {
		return err
	}

	kinds, err := s.repo.CountReactionKinds(ctx, messageID)
	if err != nil {
		return domain.Internal(err)
	}
	if kinds >= MaxEmojiKinds && !s.alreadyReacted(ctx, messageID, emoji) {
		return domain.Invalid("a message carries at most %d different reactions", MaxEmojiKinds)
	}

	added, err := s.repo.AddReaction(ctx, dbgen.AddReactionParams{
		MessageID: messageID, UserID: userID, Emoji: emoji,
	})
	if err != nil {
		return domain.Internal(err)
	}
	if added == 0 {
		return nil
	}

	s.pub.ToGuild(ctx, channel.GuildID, events.EventReactionAdd, events.MessageReaction{
		MessageID: msg.ID, ChannelID: msg.ChannelID, UserID: userID, Emoji: emoji,
	})
	return nil
}

func (s *Service) Unreact(ctx context.Context, userID, messageID uuid.UUID, emoji string) error {
	if err := validEmoji(emoji); err != nil {
		return err
	}

	msg, channel, err := s.reactable(ctx, userID, messageID, domain.PermViewChannel)
	if err != nil {
		return err
	}

	removed, err := s.repo.RemoveReaction(ctx, dbgen.RemoveReactionParams{
		MessageID: messageID, UserID: userID, Emoji: emoji,
	})
	if err != nil {
		return domain.Internal(err)
	}
	if removed == 0 {
		return nil
	}

	s.pub.ToGuild(ctx, channel.GuildID, events.EventReactionRemove, events.MessageReaction{
		MessageID: msg.ID, ChannelID: msg.ChannelID, UserID: userID, Emoji: emoji,
	})
	return nil
}

func (s *Service) Reactors(ctx context.Context, userID, messageID uuid.UUID, emoji string) ([]events.User, error) {
	if err := validEmoji(emoji); err != nil {
		return nil, err
	}
	if _, _, err := s.reactable(ctx, userID, messageID, domain.PermViewChannel); err != nil {
		return nil, err
	}

	rows, err := s.repo.ListReactors(ctx, dbgen.ListReactorsParams{MessageID: messageID, Emoji: emoji})
	if err != nil {
		return nil, domain.Internal(err)
	}

	people := make([]events.User, len(rows))
	for i, r := range rows {
		people[i] = events.User{ID: r.ID, Username: r.Username, AvatarKey: r.AvatarKey}
	}
	return people, nil
}

func (s *Service) reactable(ctx context.Context, userID, messageID uuid.UUID, want domain.Permission) (dbgen.Message, dbgen.Channel, error) {
	msg, err := s.repo.GetMessage(ctx, messageID)
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.Message{}, dbgen.Channel{}, domain.NotFound("message")
		}
		return dbgen.Message{}, dbgen.Channel{}, domain.Internal(err)
	}

	perms, channel, err := s.authz.PermissionsIn(ctx, userID, msg.ChannelID)
	if err != nil {
		return dbgen.Message{}, dbgen.Channel{}, err
	}
	if !perms.Has(domain.PermViewChannel) {
		return dbgen.Message{}, dbgen.Channel{}, domain.Forbidden("missing ViewChannel permission")
	}
	if !perms.Has(want) {
		return dbgen.Message{}, dbgen.Channel{}, domain.Forbidden("missing AddReactions permission")
	}
	return msg, channel, nil
}

func (s *Service) alreadyReacted(ctx context.Context, messageID uuid.UUID, emoji string) bool {
	rows, err := s.repo.ListReactionsForMessages(ctx, dbgen.ListReactionsForMessagesParams{
		MessageIds: []uuid.UUID{messageID},
	})
	if err != nil {
		return false
	}
	for _, r := range rows {
		if r.Emoji == emoji {
			return true
		}
	}
	return false
}

func (s *Service) attachReactions(ctx context.Context, viewerID uuid.UUID, msgs []events.Message, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.repo.ListReactionsForMessages(ctx, dbgen.ListReactionsForMessagesParams{
		ViewerID: viewerID, MessageIds: ids,
	})
	if err != nil {
		return domain.Internal(err)
	}
	if len(rows) == 0 {
		return nil
	}

	byMessage := make(map[uuid.UUID][]events.Reaction, len(rows))
	for _, r := range rows {
		byMessage[r.MessageID] = append(byMessage[r.MessageID], events.Reaction{
			Emoji: r.Emoji, Count: r.Count, Mine: r.Mine,
		})
	}
	for i := range msgs {
		if got, ok := byMessage[msgs[i].ID]; ok {
			msgs[i].Reactions = got
		}
	}
	return nil
}
