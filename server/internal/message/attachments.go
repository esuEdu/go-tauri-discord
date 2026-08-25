package message

import (
	"context"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/storage"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const (
	MaxAttachmentBytes = 25 << 20
	MaxAttachments     = 10
	MaxFilenameLen     = 200
)

type Upload struct {
	Filename    string
	ContentType string
	Body        io.Reader
}

type Signer interface {
	SignedURL(path string) string
}

func (s *Service) AttachFiles(store storage.Store, signer Signer) {
	s.store = store
	s.signer = signer
}

func attachmentURL(id uuid.UUID) string {
	return "/api/v1/attachments/" + id.String()
}

func safeFilename(given string) string {
	name := path.Base(strings.ReplaceAll(given, "\\", "/"))
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)

	if name == "" || name == "." || name == ".." {
		return "file"
	}
	if len(name) > MaxFilenameLen {
		name = name[:MaxFilenameLen]
	}
	return name
}

func (s *Service) storeUploads(ctx context.Context, uploads []Upload) ([]dbgen.CreateAttachmentParams, error) {
	stored := make([]dbgen.CreateAttachmentParams, 0, len(uploads))

	for _, upload := range uploads {
		name := safeFilename(upload.Filename)
		contentType := upload.ContentType
		if contentType == "" {
			contentType = storage.ContentTypeOf(name)
		}

		key := "attachments/" + uuid.Must(uuid.NewV7()).String() + strings.ToLower(path.Ext(name))
		if !storage.ValidKey(key) {
			key = "attachments/" + uuid.Must(uuid.NewV7()).String()
		}

		counted := &countingReader{inner: io.LimitReader(upload.Body, MaxAttachmentBytes+1)}
		if err := s.store.Put(ctx, key, contentType, counted, -1); err != nil {
			s.discard(ctx, stored)
			return nil, domain.Internal(err)
		}
		if counted.n > MaxAttachmentBytes {
			_ = s.store.Delete(ctx, key)
			s.discard(ctx, stored)
			return nil, domain.Invalid("%q is larger than the %d MB limit", name, MaxAttachmentBytes>>20)
		}

		stored = append(stored, dbgen.CreateAttachmentParams{
			ID:          uuid.Must(uuid.NewV7()),
			StorageKey:  key,
			Filename:    name,
			SizeBytes:   counted.n,
			ContentType: contentType,
		})
	}
	return stored, nil
}

func (s *Service) discard(ctx context.Context, stored []dbgen.CreateAttachmentParams) {
	for _, a := range stored {
		if err := s.store.Delete(ctx, a.StorageKey); err != nil {
			slog.WarnContext(ctx, "an abandoned upload was left behind",
				"key", a.StorageKey, "error", err)
		}
	}
}

func (s *Service) sweep(ctx context.Context, messageID uuid.UUID) {
	gone, err := s.repo.DeleteAttachmentsForMessage(ctx, messageID)
	if err != nil {
		slog.WarnContext(ctx, "could not list attachments of a deleted message",
			"message_id", messageID, "error", err)
		return
	}
	for _, a := range gone {
		if err := s.store.Delete(ctx, a.StorageKey); err != nil {
			slog.WarnContext(ctx, "a deleted message left its file behind",
				"key", a.StorageKey, "error", err)
		}
	}
}

func (s *Service) publicAttachment(a dbgen.Attachment) events.Attachment {
	url := attachmentURL(a.ID)
	if s.signer != nil {
		url = s.signer.SignedURL(url)
	}
	return events.Attachment{
		ID:          a.ID,
		Filename:    a.Filename,
		SizeBytes:   a.SizeBytes,
		ContentType: a.ContentType,
		URL:         url,
	}
}

func (s *Service) Attachment(ctx context.Context, id uuid.UUID) (storage.Store, dbgen.Attachment, error) {
	row, err := s.repo.GetAttachment(ctx, id)
	if err != nil {
		return nil, dbgen.Attachment{}, domain.NotFound("attachment")
	}
	return s.store, row, nil
}

type countingReader struct {
	inner io.Reader
	n     int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	read, err := c.inner.Read(p)
	c.n += int64(read)
	return read, err
}
