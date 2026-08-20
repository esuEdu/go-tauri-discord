package voice

import (
	"testing"

	"github.com/google/uuid"
)

func TestTrackNameSpellsOutTheOwner(t *testing.T) {
	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	cases := []struct {
		source Source
		want   string
	}{
		{SourceMicrophone, "mic-11111111-2222-3333-4444-555555555555-42"},
		{SourceScreen, "screen-11111111-2222-3333-4444-555555555555-42"},
	}

	for _, c := range cases {
		if got := TrackName(c.source, userID, 42); got != c.want {
			t.Errorf("TrackName(%q) = %q, want %q; the client splits this string to attribute media",
				c.source, got, c.want)
		}
	}
}

func TestParseTrackNameRecoversWhatTrackNameWrote(t *testing.T) {
	userID := uuid.New()

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
	for _, name := range []string{"", "audio", "mic-not-a-uuid-1", "vocalis"} {
		if _, _, ok := ParseTrackName(name); ok {
			t.Errorf("ParseTrackName(%q) claimed to find an owner", name)
		}
	}
}
