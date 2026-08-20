package voice

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Source string

const (
	SourceMicrophone Source = "mic"
	SourceScreen     Source = "screen"
)

func TrackName(source Source, userID uuid.UUID, ssrc uint32) string {
	return fmt.Sprintf("%s-%s-%d", source, userID, ssrc)
}

func ParseTrackName(name string) (Source, uuid.UUID, bool) {
	source, rest, ok := strings.Cut(name, "-")
	if !ok {
		return "", uuid.Nil, false
	}

	owner := rest
	if cut := strings.LastIndex(rest, "-"); cut >= 0 {
		owner = rest[:cut]
	}

	userID, err := uuid.Parse(owner)
	if err != nil {
		return "", uuid.Nil, false
	}
	return Source(source), userID, true
}
