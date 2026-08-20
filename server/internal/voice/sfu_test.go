package voice

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type recordingSignaler struct {
	mu     sync.Mutex
	offers map[uuid.UUID]webrtc.SessionDescription
	mids   map[uuid.UUID]*string
}

func newRecordingSignaler() *recordingSignaler {
	return &recordingSignaler{
		offers: make(map[uuid.UUID]webrtc.SessionDescription),
		mids:   make(map[uuid.UUID]*string),
	}
}

func (r *recordingSignaler) SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription, screenMid *string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offers[userID] = sdp
	r.mids[userID] = screenMid
}

func (r *recordingSignaler) SendCandidate(uuid.UUID, webrtc.ICECandidateInit) {}
func (r *recordingSignaler) VoiceClosed(uuid.UUID)                            {}
func (r *recordingSignaler) ScreenChanged(uuid.UUID, uuid.UUID, string, bool) {}

func (r *recordingSignaler) offerFor(t *testing.T, userID uuid.UUID) (webrtc.SessionDescription, *string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()

	sdp, ok := r.offers[userID]
	if !ok {
		t.Fatalf("no offer was sent to %s", userID)
	}
	return sdp, r.mids[userID]
}

func videoSections(sdp string) int {
	return strings.Count(sdp, "m=video")
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

	offer, mid := signaler.offerFor(t, userID)
	if got := videoSections(offer.SDP); got != 0 {
		t.Errorf("offer carried %d video sections, want 0", got)
	}
	if mid != nil {
		t.Errorf("offer carried screen mid %q, want none", *mid)
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

	offer, mid := signaler.offerFor(t, userID)
	if got := videoSections(offer.SDP); got != 1 {
		t.Errorf("offer carried %d video sections, want 1", got)
	}
	if mid == nil {
		t.Fatal("offer carried no screen mid, so a client cannot find the ingest transceiver")
	}
	if !strings.Contains(offer.SDP, "a=mid:"+*mid) {
		t.Errorf("screen mid %q does not appear in the offer", *mid)
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
