package storage

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("storage: no such object")

var ErrBadKey = errors.New("storage: key is not a safe object name")

type Store interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

var extensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func ExtensionFor(contentType string) (string, bool) {
	ext, ok := extensions[contentType]
	return ext, ok
}

func ContentTypeOf(key string) string {
	ext := strings.ToLower(path.Ext(key))
	for contentType, candidate := range extensions {
		if candidate == ext {
			return contentType
		}
	}
	return "application/octet-stream"
}

func NewKey(prefix, contentType string) (string, error) {
	ext, ok := ExtensionFor(contentType)
	if !ok {
		return "", ErrBadKey
	}
	return prefix + "/" + uuid.Must(uuid.NewV7()).String() + ext, nil
}

func ValidKey(key string) bool {
	if key == "" || len(key) > 200 {
		return false
	}
	if key != path.Clean(key) || path.IsAbs(key) {
		return false
	}
	if strings.HasPrefix(key, ".") || strings.Contains(key, "..") {
		return false
	}

	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '/', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
