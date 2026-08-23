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
}

func newRecordingSignaler() *recordingSignaler {
	return &recordingSignaler{offers: make(map[uuid.UUID]webrtc.SessionDescription)}
}

func (r *recordingSignaler) SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offers[userID] = sdp
}

func (r *recordingSignaler) SendCandidate(uuid.UUID, webrtc.ICECandidateInit) {}
func (r *recordingSignaler) VoiceClosed(uuid.UUID)                            {}
func (r *recordingSignaler) ScreenChanged(uuid.UUID, uuid.UUID, string, bool) {}

type recordedOffer struct {
	sdp webrtc.SessionDescription
}

func (r *recordingSignaler) offerFor(t *testing.T, userID uuid.UUID) recordedOffer {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()

	sdp, ok := r.offers[userID]
	if !ok {
		t.Fatalf("no offer was sent to %s", userID)
	}
	return recordedOffer{sdp: sdp}
}

func videoSections(sdp string) int {
	return strings.Count(sdp, "m=video")
}

func audioSections(sdp string) int {
	return strings.Count(sdp, "m=audio")
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

func TestMuteIsRememberedForWhoeverJoinsNext(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, false); err != nil {
		t.Fatalf("join: %v", err)
	}

	if muted := sfu.Muted(channelID); muted[userID] {
		t.Fatal("a member arrived already muted")
	}

	if err := sfu.SetMuted(userID, true); err != nil {
		t.Fatalf("set muted: %v", err)
	}
	if muted := sfu.Muted(channelID); !muted[userID] {
		t.Error("mute was not remembered, so anyone joining afterwards is told this member is " +
			"live when they are not; the state has to outlast the announcement that set it")
	}

	if err := sfu.SetMuted(userID, false); err != nil {
		t.Fatalf("unset muted: %v", err)
	}
	if muted := sfu.Muted(channelID); muted[userID] {
		t.Error("unmuting left the member muted")
	}
}

func TestMutingSomebodyWhoIsNotInACallFails(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.SetMuted(uuid.New(), true); err != ErrNotConnected {
		t.Errorf("SetMuted error = %v, want %v", err, ErrNotConnected)
	}
}

func TestLeavingForgetsTheMute(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, false); err != nil {
		t.Fatalf("join: %v", err)
	}
	if err := sfu.SetMuted(userID, true); err != nil {
		t.Fatalf("set muted: %v", err)
	}

	sfu.Leave(userID)
	if err := sfu.Join(channelID, userID, false); err != nil {
		t.Fatalf("rejoin: %v", err)
	}

	if muted := sfu.Muted(channelID); muted[userID] {
		t.Error("a member who left muted came back muted, while their microphone came back live; " +
			"the two would disagree and nothing would correct it until they toggled")
	}
}

func TestDroppingAShareOnlyDropsThatPersonsScreen(t *testing.T) {
	sharer, other := uuid.New(), uuid.New()
	r := &room{layers: make(map[string]layer)}
	p := &peer{ignored: map[uuid.UUID]bool{sharer: true}, sizes: map[uuid.UUID]string{}}

	cases := []struct {
		track string
		want  bool
		why   string
	}{
		{TrackName(SourceScreen, sharer, 1), false, "the screen that was dropped is still sent"},
		{TrackName(SourceScreen, other, 2), true, "dropping one screen took another with it"},
		{TrackName(SourceMicrophone, sharer, 3), true,
			"dropping a screen silenced the person sharing it, so closing a tile leaves you unable to hear them"},
		{TrackName(SourceScreenAudio, sharer, 4), true,
			"dropping a screen took its sound too; the sound is cheap and has a volume control of its own"},
		{"something-else", true, "a track this package did not name was withheld"},
	}

	for _, c := range cases {
		if got := p.wants(r, c.track); got != c.want {
			t.Errorf("wants(%q) = %v, want %v: %s", c.track, got, c.want, c.why)
		}
	}
}

func TestAViewerWhoDroppedNothingWantsEverything(t *testing.T) {
	r := &room{layers: make(map[string]layer)}
	p := &peer{ignored: map[uuid.UUID]bool{}, sizes: map[uuid.UUID]string{}}

	if !p.wants(r, TrackName(SourceScreen, uuid.New(), 1)) {
		t.Error("a viewer who dropped nothing was refused a screen")
	}
}

func TestWatchingSomebodyWhileNotInACallFails(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.SetWatching(uuid.New(), uuid.New(), false, ""); err != ErrNotConnected {
		t.Errorf("SetWatching error = %v, want %v", err, ErrNotConnected)
	}
}

func TestDroppingAndResumingAShareIsRemembered(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer, sharer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, true); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.SetWatching(viewer, sharer, false, ""); err != nil {
		t.Fatalf("stop watching: %v", err)
	}

	sfu.mu.Lock()
	dropped := sfu.rooms[channelID].peers[viewer].ignored[sharer]
	sfu.mu.Unlock()
	if !dropped {
		t.Fatal("a dropped share was not recorded, so the next renegotiation sends it again")
	}

	if err := sfu.SetWatching(viewer, sharer, true, ""); err != nil {
		t.Fatalf("resume watching: %v", err)
	}

	sfu.mu.Lock()
	stillDropped := sfu.rooms[channelID].peers[viewer].ignored[sharer]
	sfu.mu.Unlock()
	if stillDropped {
		t.Error("resuming left the share dropped")
	}
}

func TestAViewerIsSentOneSizeOfAScreenAndNotTheOther(t *testing.T) {
	sharer, viewer := uuid.New(), uuid.New()
	full := TrackName(SourceScreen, sharer, 1)
	half := TrackName(SourceScreen, sharer, 2)

	r := &room{layers: map[string]layer{
		full: {owner: sharer, rid: DefaultLayer},
		half: {owner: sharer, rid: SmallerLayer},
	}}
	p := &peer{userID: viewer, ignored: map[uuid.UUID]bool{}, sizes: map[uuid.UUID]string{}}

	if !p.wants(r, full) || p.wants(r, half) {
		t.Fatal("a viewer who has asked for nothing is not being sent exactly one size; sending " +
			"both is the whole cost of simulcast with none of the benefit")
	}

	p.sizes[sharer] = SmallerLayer
	if p.wants(r, full) || !p.wants(r, half) {
		t.Error("asking for the smaller size did not switch which one is sent")
	}

	p.ignored[sharer] = true
	if p.wants(r, full) || p.wants(r, half) {
		t.Error("dropping a screen left one of its sizes still being sent")
	}
}

func TestAScreenWithNoLayersIsStillSent(t *testing.T) {
	sharer := uuid.New()
	only := TrackName(SourceScreen, sharer, 1)

	r := &room{layers: map[string]layer{}}
	p := &peer{ignored: map[uuid.UUID]bool{}, sizes: map[uuid.UUID]string{}}

	if !p.wants(r, only) {
		t.Error("a screen published in one size was withheld; a browser that will not do " +
			"simulcast has to keep working exactly as it did")
	}
}

func TestAskingForASmallerScreenIsRemembered(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer, sharer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, true); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.SetWatching(viewer, sharer, true, SmallerLayer); err != nil {
		t.Fatalf("ask for the smaller size: %v", err)
	}

	sfu.mu.Lock()
	chosen := sfu.rooms[channelID].peers[viewer].sizeFor(sharer)
	sfu.mu.Unlock()
	if chosen != SmallerLayer {
		t.Fatalf("chosen size = %q, want %q", chosen, SmallerLayer)
	}

	if err := sfu.SetWatching(viewer, sharer, true, "enormous"); err != nil {
		t.Fatalf("ask for a size that does not exist: %v", err)
	}

	sfu.mu.Lock()
	after := sfu.rooms[channelID].peers[viewer].sizeFor(sharer)
	sfu.mu.Unlock()
	if after != SmallerLayer {
		t.Errorf("a size the server does not publish overwrote a real choice: %q", after)
	}
}

func TestTakingAShareBackKeepsTheSizeYouChose(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer, sharer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, true); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.SetWatching(viewer, sharer, true, SmallerLayer); err != nil {
		t.Fatalf("choose smaller: %v", err)
	}
	if err := sfu.SetWatching(viewer, sharer, false, ""); err != nil {
		t.Fatalf("stop watching: %v", err)
	}
	if err := sfu.SetWatching(viewer, sharer, true, ""); err != nil {
		t.Fatalf("watch again: %v", err)
	}

	sfu.mu.Lock()
	chosen := sfu.rooms[channelID].peers[viewer].sizeFor(sharer)
	sfu.mu.Unlock()
	if chosen != SmallerLayer {
		t.Error("watching again reset somebody to the full size, so a viewer who chose smaller " +
			"because their connection could not take it is handed the large one the moment they " +
			"look away and back")
	}
}

func TestAMemberWhoMayNotStreamCannotPublishAScreen(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	sfu.AttachPublishSignaler(&recordingPublishSignaler{
		answers: make(chan webrtc.SessionDescription, 1),
	})

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, false); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.PublishScreen(userID, webrtc.SessionDescription{}); err != ErrNotAllowed {
		t.Fatalf("PublishScreen error = %v, want %v; the video section used to be the refusal "+
			"and there is no video section any more, so this check is now the whole of it", err, ErrNotAllowed)
	}
}

func TestJoiningOffersOnlyAMicrophone(t *testing.T) {
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
	if got := videoSections(offer.sdp.SDP); got != 0 {
		t.Errorf("offer carried %d video sections, want none; a screen has its own connection now", got)
	}
	if got := audioSections(offer.sdp.SDP); got != 1 {
		t.Errorf("offer carried %d audio sections, want 1 for the microphone", got)
	}
}

func TestAStaleCloseDoesNotEvictSomebodyWhoRejoined(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, true); err != nil {
		t.Fatalf("join: %v", err)
	}
	first := sfu.peerFor(userID)
	if first == nil {
		t.Fatal("joining left nobody in the room")
	}

	if err := sfu.Join(channelID, userID, true); err != nil {
		t.Fatalf("rejoin: %v", err)
	}
	second := sfu.peerFor(userID)
	if second == nil || second == first {
		t.Fatal("rejoining did not replace the peer, so this proves nothing")
	}

	sfu.leave(userID, first)

	if got := sfu.peerFor(userID); got != second {
		t.Error("the closing of an old connection evicted the member who had already rejoined on a " +
			"new one; pion fires that close asynchronously, so the eviction lands whenever it lands " +
			"and the member is simply gone from a call they are sitting in")
	}
}

func TestLeavingTakesTheScreenConnectionWithIt(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	signals := &recordingPublishSignaler{answers: make(chan webrtc.SessionDescription, 1)}
	sfu.AttachPublishSignaler(signals)

	userID := uuid.New()
	if err := sfu.Join(uuid.New(), userID, true); err != nil {
		t.Fatalf("join: %v", err)
	}
	publishOneScreen(t, sfu, userID)

	sfu.publishMu.Lock()
	published := sfu.publishers[userID]
	sfu.publishMu.Unlock()
	if published == nil {
		t.Fatal("publishing recorded no connection, so this proves nothing")
	}

	sfu.Leave(userID)

	sfu.publishMu.Lock()
	left := sfu.publishers[userID]
	sfu.publishMu.Unlock()
	if left != nil {
		t.Error("leaving the call left the screen connection open and forgotten; nothing else ever " +
			"closes it, so every share ever started outlives its call and holds its ICE and DTLS open")
	}
	if state := published.pc.ConnectionState(); state != webrtc.PeerConnectionStateClosed {
		t.Errorf("the abandoned screen connection is %s, want closed", state)
	}
}
