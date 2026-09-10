package voice

import (
	"fmt"
	"strings"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Source string

const (
	SourceMicrophone  Source = "mic"
	SourceScreen      Source = "screen"
	SourceScreenAudio Source = "screenaudio"
)

func TrackName(source Source, userID events.UserID, ssrc uint32) string {
	return fmt.Sprintf("%s-%s-%d", source, userID, ssrc)
}

func ParseTrackName(name string) (Source, events.UserID, bool) {
	source, rest, ok := strings.Cut(name, "-")
	if !ok {
		return "", "", false
	}

	owner := rest
	if cut := strings.LastIndex(rest, "-"); cut >= 0 {
		owner = rest[:cut]
	}
	if !ValidPublicID(owner) {
		return "", "", false
	}
	return Source(source), events.UserID(owner), true
}

const publicIDLen = 16

func ValidPublicID(s string) bool {
	if len(s) != publicIDLen {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '2' || r > '7') {
			return false
		}
	}
	return true
}

const (
	DefaultLayer = "full"
	SmallerLayer = "half"
)

func KnownLayer(rid string) bool {
	return rid == DefaultLayer || rid == SmallerLayer
}
