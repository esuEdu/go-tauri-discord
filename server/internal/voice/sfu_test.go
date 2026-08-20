package voice

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type recordingSignaler struct {
	mu        sync.Mutex
	offers    map[uuid.UUID]webrtc.SessionDescription
	mids      map[uuid.UUID]*string
	audioMids map[uuid.UUID]*string
}

func newRecordingSignaler() *recordingSignaler {
	return &recordingSignaler{
		offers:    make(map[uuid.UUID]webrtc.SessionDescription),
		mids:      make(map[uuid.UUID]*string),
		audioMids: make(map[uuid.UUID]*string),
	}
}

func (r *recordingSignaler) SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription, screenMid, screenAudioMid *string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offers[userID] = sdp
	r.mids[userID] = screenMid
	r.audioMids[userID] = screenAudioMid
}

func (r *recordingSignaler) SendCandidate(uuid.UUID, webrtc.ICECandidateInit) {}
func (r *recordingSignaler) VoiceClosed(uuid.UUID)                            {}
func (r *recordingSignaler) ScreenChanged(uuid.UUID, uuid.UUID, string, bool) {}

type recordedOffer struct {
	sdp         webrtc.SessionDescription
	screen      *string
	screenAudio *string
}

func (r *recordingSignaler) offerFor(t *testing.T, userID uuid.UUID) recordedOffer {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()

	sdp, ok := r.offers[userID]
	if !ok {
		t.Fatalf("no offer was sent to %s", userID)
	}
	return recordedOffer{sdp: sdp, screen: r.mids[userID], screenAudio: r.audioMids[userID]}
}

func videoSections(sdp string) int {
	return strings.Count(sdp, "m=video")
}

func audioSections(sdp string) int {
	return strings.Count(sdp, "m=audio")
}

func TestJoinWithoutStreamPermissionOffersNoVideo(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	userID := uuid.New()
	if err := sfu.Join(uuid.New(), userID, false); err != nil {
		t.Fatalf("join: %v", err)
	}

	offer := signaler.offerFor(t, userID)
	if got := videoSections(offer.sdp.SDP); got != 0 {
		t.Errorf("offer carried %d video sections, want 0", got)
	}
	if offer.screen != nil {
		t.Errorf("offer carried screen mid %q, want none", *offer.screen)
	}
	if got := audioSections(offer.sdp.SDP); got != 1 {
		t.Errorf("offer carried %d audio sections, want 1 for the microphone alone", got)
	}
	if offer.screenAudio != nil {
		t.Errorf("offer carried screen audio mid %q, so a member who may not stream was "+
			"handed a slot to stream sound through", *offer.screenAudio)
	}
}

func TestJoinWithStreamPermissionOffersAScreenSlot(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	userID := uuid.New()
	if err := sfu.Join(uuid.New(), userID, true); err != nil {
		t.Fatalf("join: %v", err)
	}

	offer := signaler.offerFor(t, userID)
	if got := videoSections(offer.sdp.SDP); got != 1 {
		t.Errorf("offer carried %d video sections, want 1", got)
	}
	if offer.screen == nil {
		t.Fatal("offer carried no screen mid, so a client cannot find the ingest transceiver")
	}
	if !strings.Contains(offer.sdp.SDP, "a=mid:"+*offer.screen) {
		t.Errorf("screen mid %q does not appear in the offer", *offer.screen)
	}
}

func TestJoinWithStreamPermissionOffersASlotForScreenSound(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	userID := uuid.New()
	if err := sfu.Join(uuid.New(), userID, true); err != nil {
		t.Fatalf("join: %v", err)
	}

	offer := signaler.offerFor(t, userID)
	if got := audioSections(offer.sdp.SDP); got != 2 {
		t.Errorf("offer carried %d audio sections, want 2: one microphone and one screen", got)
	}
	if offer.screenAudio == nil {
		t.Fatal("offer carried no screen audio mid, so a sharer has nowhere to put the sound " +
			"of what it is showing")
	}
	if offer.screen != nil && *offer.screenAudio == *offer.screen {
		t.Fatalf("screen and screen audio share mid %q; sound would replace the picture",
			*offer.screenAudio)
	}
	if !strings.Contains(offer.sdp.SDP, "a=mid:"+*offer.screenAudio) {
		t.Errorf("screen audio mid %q does not appear in the offer", *offer.screenAudio)
	}
}

func TestResyncOnAnUnknownUserFails(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.Resync(uuid.New()); err != ErrNotConnected {
		t.Errorf("resync error = %v, want %v", err, ErrNotConnected)
	}
}
