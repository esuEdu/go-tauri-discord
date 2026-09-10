package auth

import (
	"crypto/rand"
	"encoding/base32"
	"strings"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const publicIDBytes = 10

var publicIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func newPublicID() (events.UserID, error) {
	buf := make([]byte, publicIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", domain.Internal(err)
	}
	return events.UserID(strings.ToLower(publicIDEncoding.EncodeToString(buf))), nil
}
