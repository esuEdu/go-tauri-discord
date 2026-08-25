package message

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/bus"
	"github.com/esuEdu/go-tauri-discord/internal/storage"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Repository interface {
	CreateMessage(ctx context.Context, arg dbgen.CreateMessageParams) (dbgen.Message, error)
	GetMessage(ctx context.Context, id uuid.UUID) (dbgen.Message, error)
	ListMessages(ctx context.Context, arg dbgen.ListMessagesParams) ([]dbgen.ListMessagesRow, error)
	UpdateMessageContent(ctx context.Context, arg dbgen.UpdateMessageContentParams) (dbgen.Message, error)
	SoftDeleteMessage(ctx context.Context, id uuid.UUID) error
	ListAttachmentsForMessages(ctx context.Context, messageIDs []uuid.UUID) ([]dbgen.Attachment, error)
	CreateAttachment(ctx context.Context, arg dbgen.CreateAttachmentParams) (dbgen.Attachment, error)
	GetAttachment(ctx context.Context, id uuid.UUID) (dbgen.Attachment, error)
	DeleteAttachmentsForMessage(ctx context.Context, messageID uuid.UUID) ([]dbgen.Attachment, error)
	AddReaction(ctx context.Context, arg dbgen.AddReactionParams) (int64, error)
	RemoveReaction(ctx context.Context, arg dbgen.RemoveReactionParams) (int64, error)
	CountReactionKinds(ctx context.Context, messageID uuid.UUID) (int32, error)
	ListReactionsForMessages(ctx context.Context, arg dbgen.ListReactionsForMessagesParams) ([]dbgen.ListReactionsForMessagesRow, error)
	ListReactors(ctx context.Context, arg dbgen.ListReactorsParams) ([]dbgen.ListReactorsRow, error)
	UpsertReadState(ctx context.Context, arg dbgen.UpsertReadStateParams) error
	ListReadStates(ctx context.Context, userID uuid.UUID) ([]dbgen.ReadState, error)
	ListLatestMessageIDs(ctx context.Context, channelIDs []uuid.UUID) ([]dbgen.ListLatestMessageIDsRow, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error)
}

type Authorizer interface {
	PermissionsIn(ctx context.Context, userID, channelID uuid.UUID) (domain.Permission, dbgen.Channel, error)
}

type Service struct {
	repo   Repository
	authz  Authorizer
	pub    *bus.Publisher
	store  storage.Store
	signer Signer
}

func NewService(repo Repository, authz Authorizer, pub *bus.Publisher) *Service {
	return &Service{repo: repo, authz: authz, pub: pub}
}

const (
	MaxContentLen   = 4000
	DefaultPageSize = 50
	MaxPageSize     = 100
)

func (s *Service) Create(ctx context.Context, userID, channelID uuid.UUID, content string, uploads ...Upload) (events.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" && len(uploads) == 0 {
		return events.Message{}, domain.Invalid("message content is required")
	}
	if len(uploads) > MaxAttachments {
		return events.Message{}, domain.Invalid("a message carries at most %d files", MaxAttachments)
	}
	if len(uploads) > 0 && s.store == nil {
		return events.Message{}, domain.Invalid("this server has file uploads turned off")
	}
	if utf8.RuneCountInString(content) > MaxContentLen {
		return events.Message{}, domain.Invalid("message exceeds %d characters", MaxContentLen)
	}

	perms, channel, err := s.authz.PermissionsIn(ctx, userID, channelID)
	if err != nil {
		return events.Message{}, err
	}
	if channel.Kind != domain.ChannelText {
		return events.Message{}, domain.Invalid("cannot post to a %s channel", channel.Kind)
	}
	if !perms.Has(domain.PermViewChannel | domain.PermSendMessages) {
		return events.Message{}, domain.Forbidden("missing SendMessages permission")
	}

	author, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return events.Message{}, domain.Internal(err)
	}

	stored, err := s.storeUploads(ctx, uploads)
	if err != nil {
		return events.Message{}, err
	}

	row, err := s.repo.CreateMessage(ctx, dbgen.CreateMessageParams{
		ID:        uuid.Must(uuid.NewV7()),
		ChannelID: channelID,
		AuthorID:  userID,
		Content:   content,
	})
	if err != nil {
		s.discard(ctx, stored)
		return events.Message{}, domain.Internal(err)
	}

	attached := make([]events.Attachment, 0, len(stored))
	for _, params := range stored {
		params.MessageID = row.ID
		saved, err := s.repo.CreateAttachment(ctx, params)
		if err != nil {
			s.discard(ctx, stored)
			_ = s.repo.SoftDeleteMessage(ctx, row.ID)
			return events.Message{}, domain.Internal(err)
		}
		attached = append(attached, s.publicAttachment(saved))
	}

	msg := events.Message{
		ID:          row.ID,
		ChannelID:   row.ChannelID,
		Author:      events.User{ID: author.ID, Username: author.Username, AvatarKey: author.AvatarKey},
		Content:     row.Content,
		CreatedAt:   row.CreatedAt,
		EditedAt:    row.EditedAt,
		Attachments: attached,
		Reactions:   []events.Reaction{},
	}

	s.pub.ToGuild(ctx, channel.GuildID, events.EventMessageCreate, msg)
	return msg, nil
}

func (s *Service) History(ctx context.Context, userID, channelID uuid.UUID, before *uuid.UUID, limit int) ([]events.Message, error) {
	perms, _, err := s.authz.PermissionsIn(ctx, userID, channelID)
	if err != nil {
		return nil, err
	}
	if !perms.Has(domain.PermViewChannel) {
		return nil, domain.Forbidden("missing ViewChannel permission")
	}

	if limit <= 0 || limit > MaxPageSize {
		limit = DefaultPageSize
	}
	rows, err := s.repo.ListMessages(ctx, dbgen.ListMessagesParams{
		ChannelID: channelID,
		Before:    before,
		PageSize:  int32(limit),
	})
	if err != nil {
		return nil, domain.Internal(err)
	}

	out := make([]events.Message, len(rows))
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		out[i] = events.Message{
			ID:        r.ID,
			ChannelID: r.ChannelID,
			Author: events.User{
				ID:        r.AuthorID,
				Username:  r.AuthorUsername,
				AvatarKey: r.AuthorAvatarKey,
			},
			Content:     r.Content,
			CreatedAt:   r.CreatedAt,
			EditedAt:    r.EditedAt,
			Attachments: []events.Attachment{},
			Reactions:   []events.Reaction{},
		}
	}

	if err := s.attachAttachments(ctx, out, ids); err != nil {
		return nil, err
	}
	if err := s.attachReactions(ctx, userID, out, ids); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) attachAttachments(ctx context.Context, msgs []events.Message, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.repo.ListAttachmentsForMessages(ctx, ids)
	if err != nil {
		return domain.Internal(err)
	}
	if len(rows) == 0 {
		return nil
	}

	byMessage := make(map[uuid.UUID][]events.Attachment, len(rows))
	for _, a := range rows {
		byMessage[a.MessageID] = append(byMessage[a.MessageID], s.publicAttachment(a))
	}
	for i := range msgs {
		if got, ok := byMessage[msgs[i].ID]; ok {
			msgs[i].Attachments = got
		}
	}
	return nil
}

func (s *Service) Edit(ctx context.Context, userID, messageID uuid.UUID, content string) (events.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return events.Message{}, domain.Invalid("message content is required")
	}
	if utf8.RuneCountInString(content) > MaxContentLen {
		return events.Message{}, domain.Invalid("message exceeds %d characters", MaxContentLen)
	}

	existing, err := s.repo.GetMessage(ctx, messageID)
	if err != nil {
		if db.IsNoRows(err) {
			return events.Message{}, domain.NotFound("message")
		}
		return events.Message{}, domain.Internal(err)
	}
	if existing.AuthorID != userID {
		return events.Message{}, domain.Forbidden("only the author can edit a message")
	}

	perms, channel, err := s.authz.PermissionsIn(ctx, userID, existing.ChannelID)
	if err != nil {
		return events.Message{}, err
	}
	if !perms.Has(domain.PermSendMessages) {
		return events.Message{}, domain.Forbidden("missing SendMessages permission")
	}

	row, err := s.repo.UpdateMessageContent(ctx, dbgen.UpdateMessageContentParams{
		ID: messageID, Content: content,
	})
	if err != nil {
		return events.Message{}, domain.Internal(err)
	}

	author, err := s.repo.GetUserByID(ctx, row.AuthorID)
	if err != nil {
		return events.Message{}, domain.Internal(err)
	}

	edited := []events.Message{{
		ID:          row.ID,
		ChannelID:   row.ChannelID,
		Author:      events.User{ID: author.ID, Username: author.Username, AvatarKey: author.AvatarKey},
		Content:     row.Content,
		CreatedAt:   row.CreatedAt,
		EditedAt:    row.EditedAt,
		Attachments: []events.Attachment{},
		Reactions:   []events.Reaction{},
	}}
	if err := s.attachAttachments(ctx, edited, []uuid.UUID{row.ID}); err != nil {
		return events.Message{}, err
	}

	s.pub.ToGuild(ctx, channel.GuildID, events.EventMessageUpdate, edited[0])
	return edited[0], nil
}

func (s *Service) Delete(ctx context.Context, userID, messageID uuid.UUID) error {
	existing, err := s.repo.GetMessage(ctx, messageID)
	if err != nil {
		if db.IsNoRows(err) {
			return domain.NotFound("message")
		}
		return domain.Internal(err)
	}

	perms, channel, err := s.authz.PermissionsIn(ctx, userID, existing.ChannelID)
	if err != nil {
		return err
	}
	if existing.AuthorID != userID && !perms.Has(domain.PermManageMessages) {
		return domain.Forbidden("missing ManageMessages permission")
	}

	if err := s.repo.SoftDeleteMessage(ctx, messageID); err != nil {
		return domain.Internal(err)
	}
	if s.store != nil {
		s.sweep(ctx, messageID)
	}

	s.pub.ToGuild(ctx, channel.GuildID, events.EventMessageDelete, events.MessageDelete{
		ID: messageID, ChannelID: existing.ChannelID,
	})
	return nil
}

func (s *Service) Typing(ctx context.Context, userID, channelID uuid.UUID) error {
	perms, channel, err := s.authz.PermissionsIn(ctx, userID, channelID)
	if err != nil {
		return err
	}
	if !perms.Has(domain.PermSendMessages) {
		return domain.Forbidden("missing SendMessages permission")
	}
	s.pub.ToGuild(ctx, channel.GuildID, events.EventTypingStart, events.TypingStart{
		ChannelID: channelID, UserID: userID, Timestamp: time.Now(),
	})
	return nil
}

func (s *Service) MarkRead(ctx context.Context, userID, channelID, messageID uuid.UUID) error {
	perms, _, err := s.authz.PermissionsIn(ctx, userID, channelID)
	if err != nil {
		return err
	}
	if !perms.Has(domain.PermViewChannel) {
		return domain.Forbidden("missing ViewChannel permission")
	}
	if err := s.repo.UpsertReadState(ctx, dbgen.UpsertReadStateParams{
		UserID: userID, ChannelID: channelID, LastReadMessageID: &messageID,
	}); err != nil {
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) ReadStates(ctx context.Context, userID uuid.UUID) ([]events.ReadState, error) {
	rows, err := s.repo.ListReadStates(ctx, userID)
	if err != nil {
		return nil, domain.Internal(err)
	}

	states := make([]events.ReadState, 0, len(rows))
	for _, row := range rows {
		states = append(states, events.ReadState{
			ChannelID:         row.ChannelID,
			LastReadMessageID: row.LastReadMessageID,
		})
	}
	return states, nil
}

func (s *Service) LatestMessages(ctx context.Context, channelIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}

	rows, err := s.repo.ListLatestMessageIDs(ctx, channelIDs)
	if err != nil {
		return nil, domain.Internal(err)
	}

	latest := make(map[uuid.UUID]uuid.UUID, len(rows))
	for _, row := range rows {
		latest[row.ChannelID] = row.ID
	}
	return latest, nil
}
