package auth

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const (
	MaxCustomStatusLen = 128
	MaxBioLen          = 500
)

type ProfileEdit struct {
	Status       *string
	CustomStatus *string
	Bio          *string
}

func SelfOf(u dbgen.User) events.Self {
	return events.Self{Status: u.Status, CustomStatus: u.CustomStatus, Bio: u.Bio}
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, edit ProfileEdit) (dbgen.User, error) {
	params := dbgen.UpdateUserProfileParams{ID: userID}

	if edit.Status != nil {
		chosen, err := domain.ChosenStatus(*edit.Status)
		if err != nil {
			return dbgen.User{}, err
		}
		text := string(chosen)
		params.Status = &text
	}

	if edit.CustomStatus != nil {
		clean, err := trimmed(*edit.CustomStatus, MaxCustomStatusLen, "a status")
		if err != nil {
			return dbgen.User{}, err
		}
		if clean == "" {
			params.ClearCustomStatus = true
		} else {
			params.CustomStatus = &clean
		}
	}

	if edit.Bio != nil {
		clean, err := trimmed(*edit.Bio, MaxBioLen, "a bio")
		if err != nil {
			return dbgen.User{}, err
		}
		if clean == "" {
			params.ClearBio = true
		} else {
			params.Bio = &clean
		}
	}

	updated, err := s.repo.UpdateUserProfile(ctx, params)
	if err != nil {
		return dbgen.User{}, domain.Internal(err)
	}
	return updated, nil
}

func trimmed(raw string, limit int, what string) (string, error) {
	clean := strings.TrimSpace(raw)
	if !utf8.ValidString(clean) {
		return "", domain.Invalid("%s must be valid utf-8", what)
	}
	if utf8.RuneCountInString(clean) > limit {
		return "", domain.Invalid("%s is at most %d characters", what, limit)
	}
	for _, r := range clean {
		if r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 {
			return "", domain.Invalid("%s cannot carry control characters", what)
		}
	}
	return clean, nil
}
