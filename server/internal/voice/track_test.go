package voice

import (
	"testing"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func TestTrackNameSpellsOutTheOwner(t *testing.T) {
	userID := events.UserID("k3m9x7q2wp4rt8ab")

	cases := []struct {
		source Source
		want   string
	}{
		{SourceMicrophone, "mic-k3m9x7q2wp4rt8ab-42"},
		{SourceScreen, "screen-k3m9x7q2wp4rt8ab-42"},
	}

	for _, c := range cases {
		if got := TrackName(c.source, userID, 42); got != c.want {
			t.Errorf("TrackName(%q) = %q, want %q; the client splits this string to attribute media",
				c.source, got, c.want)
		}
	}
}

func TestParseTrackNameRecoversWhatTrackNameWrote(t *testing.T) {
	userID := events.UserID("abcdefghijklmn23")

	source, owner, ok := ParseTrackName(TrackName(SourceMicrophone, userID, 7))
	if !ok {
		t.Fatal("a name this package wrote could not be parsed back")
	}
	if source != SourceMicrophone {
		t.Errorf("source = %q, want %q", source, SourceMicrophone)
	}
	if owner != userID {
		t.Errorf("owner = %s, want %s", owner, userID)
	}
}

func TestParseTrackNameRejectsForeignNames(t *testing.T) {
	names := []string{
		"", "audio", "mic-not-a-public-id-1", "vocalis",
		"mic-11111111-2222-3333-4444-555555555555-42",
		"mic-ABCDEFGHIJKLMN23-1",
		"mic-abcdefghijklmn2-1",
	}
	for _, name := range names {
		if _, _, ok := ParseTrackName(name); ok {
			t.Errorf("ParseTrackName(%q) claimed to find an owner", name)
		}
	}
}
