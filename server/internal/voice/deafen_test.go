package voice

import (
	"testing"

	"github.com/google/uuid"
)

func TestADeafenedPeerIsSentNoSoundButKeepsThePicture(t *testing.T) {
	sharer := uuid.New()
	r := &room{layers: make(map[string]layer)}
	p := &peer{deafened: true, ignored: map[uuid.UUID]bool{}, sizes: map[uuid.UUID]string{}}

	cases := []struct {
		track string
		want  bool
		why   string
	}{
		{TrackName(SourceMicrophone, sharer, 1), false,
			"a deafened member is still sent everybody's microphone, so deafening costs them the " +
				"bandwidth it is supposed to save and only works if their client stays honest"},
		{TrackName(SourceScreenAudio, sharer, 2), false,
			"a deafened member is still sent the sound of a shared screen, which is sound like any other"},
		{TrackName(SourceScreen, sharer, 3), true,
			"deafening took the picture away too; deafening is about hearing, and somebody who " +
				"cannot hear can still watch"},
	}

	for _, c := range cases {
		if got := p.wants(r, c.track); got != c.want {
			t.Errorf("wants(%q) = %v, want %v: %s", c.track, got, c.want, c.why)
		}
	}
}

func TestHearingComesBackWhenDeafeningIsLifted(t *testing.T) {
	speaker := uuid.New()
	r := &room{layers: make(map[string]layer)}
	p := &peer{deafened: true, ignored: map[uuid.UUID]bool{}, sizes: map[uuid.UUID]string{}}

	mic := TrackName(SourceMicrophone, speaker, 1)
	if p.wants(r, mic) {
		t.Fatal("a deafened peer wants a microphone track")
	}

	p.deafened = false
	if !p.wants(r, mic) {
		t.Error("undeafening left the member silent, so the only way back to hearing anybody is to rejoin")
	}
}

func TestDeafeningSomebodyWhoIsNotInACallFails(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.SetDeafened(uuid.New(), true); err != ErrNotConnected {
		t.Errorf("SetDeafened error = %v, want %v", err, ErrNotConnected)
	}
}

func TestDeafeningIsRememberedForWhoeverJoinsNext(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, false); err != nil {
		t.Fatalf("join: %v", err)
	}
	if stateOf(t, sfu, channelID, userID).Deafened {
		t.Fatal("a member arrived already deafened")
	}

	if err := sfu.SetDeafened(userID, true); err != nil {
		t.Fatalf("set deafened: %v", err)
	}
	if !stateOf(t, sfu, channelID, userID).Deafened {
		t.Error("deafening was not remembered, so the next person to join is told this member can " +
			"hear them when they cannot")
	}
}
