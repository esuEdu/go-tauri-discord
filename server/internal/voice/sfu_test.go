package voice

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"

	"encoding/base32"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type departure struct {
	channelID uuid.UUID
	userID    events.UserID
}

type screenChange struct {
	userID   events.UserID
	streamID string
	active   bool
}

type recordingSignaler struct {
	mu         sync.Mutex
	offers     map[uuid.UUID]webrtc.SessionDescription
	sent       map[uuid.UUID]int
	departures []departure
	screens    []screenChange
}

func newRecordingSignaler() *recordingSignaler {
	return &recordingSignaler{
		offers: make(map[uuid.UUID]webrtc.SessionDescription),
		sent:   make(map[uuid.UUID]int),
	}
}

func (r *recordingSignaler) SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.offers[userID] = sdp
	r.sent[userID]++
}

func (r *recordingSignaler) offersTo(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sent[userID]
}

func (r *recordingSignaler) SendCandidate(uuid.UUID, webrtc.ICECandidateInit) {}
func (r *recordingSignaler) QualityChanged(uuid.UUID, Quality)                {}

func (r *recordingSignaler) ScreenChanged(_ uuid.UUID, userID events.UserID, streamID string, active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.screens = append(r.screens, screenChange{userID: userID, streamID: streamID, active: active})
}

func (r *recordingSignaler) sawScreen(want screenChange) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, change := range r.screens {
		if change == want {
			return true
		}
	}
	return false
}

func (r *recordingSignaler) VoiceClosed(channelID uuid.UUID, userID events.UserID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.departures = append(r.departures, departure{channelID: channelID, userID: userID})
}

func (r *recordingSignaler) departed(channelID, userID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.departures {
		if d.channelID == channelID && d.userID == publicOf(userID) {
			return true
		}
	}
	return false
}

func stateOf(t *testing.T, sfu *SFU, channelID, userID uuid.UUID) Participant {
	t.Helper()
	for _, p := range sfu.States(channelID) {
		if p.UserID == userID {
			return p
		}
	}
	t.Fatalf("no voice state for %s in %s", userID, channelID)
	return Participant{}
}

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
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.Resync(uuid.New()); err != ErrNotConnected {
		t.Errorf("resync error = %v, want %v", err, ErrNotConnected)
	}
}

func TestMuteIsRememberedForWhoeverJoinsNext(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, publicOf(userID), false); err != nil {
		t.Fatalf("join: %v", err)
	}

	if stateOf(t, sfu, channelID, userID).Muted {
		t.Fatal("a member arrived already muted")
	}

	if err := sfu.SetMuted(userID, true); err != nil {
		t.Fatalf("set muted: %v", err)
	}
	if !stateOf(t, sfu, channelID, userID).Muted {
		t.Error("mute was not remembered, so anyone joining afterwards is told this member is " +
			"live when they are not; the state has to outlast the announcement that set it")
	}

	if err := sfu.SetMuted(userID, false); err != nil {
		t.Fatalf("unset muted: %v", err)
	}
	if stateOf(t, sfu, channelID, userID).Muted {
		t.Error("unmuting left the member muted")
	}
}

func TestMutingSomebodyWhoIsNotInACallFails(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.SetMuted(uuid.New(), true); err != ErrNotConnected {
		t.Errorf("SetMuted error = %v, want %v", err, ErrNotConnected)
	}
}

func TestLeavingForgetsTheMute(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, publicOf(userID), false); err != nil {
		t.Fatalf("join: %v", err)
	}
	if err := sfu.SetMuted(userID, true); err != nil {
		t.Fatalf("set muted: %v", err)
	}

	sfu.Leave(userID)
	if err := sfu.Join(channelID, userID, publicOf(userID), false); err != nil {
		t.Fatalf("rejoin: %v", err)
	}

	if stateOf(t, sfu, channelID, userID).Muted {
		t.Error("a member who left muted came back muted, while their microphone came back live; " +
			"the two would disagree and nothing would correct it until they toggled")
	}
}

func TestResyncResendsAnOfferThatNeverArrived(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, publicOf(userID), true); err != nil {
		t.Fatalf("join: %v", err)
	}
	if got := signaler.offersTo(userID); got != 1 {
		t.Fatalf("offers after joining = %d, want 1", got)
	}

	if err := sfu.Resync(userID); err != nil {
		t.Fatalf("resync: %v", err)
	}

	if got := signaler.offersTo(userID); got < 2 {
		t.Errorf("offers after a resync = %d, want the unanswered one sent again: a socket that "+
			"drops takes any offer queued behind it with it, since control frames are not replayed, "+
			"and the peer then sits in have-local-offer where every later attempt to renegotiate "+
			"gives up; nothing new is ever heard or seen in that call again", got)
	}
}

func TestAConnectionThatClosesOnItsOwnIsAnnouncedAsALeave(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, publicOf(userID), true); err != nil {
		t.Fatalf("join: %v", err)
	}

	p := sfu.peerFor(userID)
	if p == nil {
		t.Fatal("the peer went missing right after joining")
	}
	p.pc.Close()

	deadline := time.Now().Add(5 * time.Second)
	for !signaler.departed(channelID, userID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !signaler.departed(channelID, userID) {
		t.Error("a peer whose connection closed was dropped without a word, so everyone else keeps " +
			"them in the channel; the explicit leave that follows finds nobody to remove and stays " +
			"silent too, and the ghost never goes away")
	}
	if _, still := sfu.ChannelOf(userID); still {
		t.Error("the peer is still in a channel after its connection closed")
	}
}

func TestLosingStreamTakesTheScreenDownMidShare(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, sharer := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, sharer, publicOf(sharer), true); err != nil {
		t.Fatalf("join: %v", err)
	}
	const streamID = "screen-being-shared"
	share(t, sfu, channelID, sharer, streamID)

	if err := sfu.SetMayStream(sharer, false); err != nil {
		t.Fatalf("revoke stream: %v", err)
	}

	if !signaler.sawScreen(screenChange{userID: publicOf(sharer), streamID: streamID, active: false}) {
		t.Error("stream was taken away from somebody in the middle of sharing and the screen kept " +
			"going: the permission is only read when a share starts, so a share already running " +
			"outlives the permission that allowed it")
	}
	if err := sfu.SetScreenActive(sharer, true); err != ErrNotAllowed {
		t.Errorf("re-announcing a screen after losing stream = %v, want %v; the client can put its "+
			"own tile back up by asking", err, ErrNotAllowed)
	}
}

func TestGettingStreamBackAllowsSharingAgain(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, sharer := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, sharer, publicOf(sharer), false); err != nil {
		t.Fatalf("join: %v", err)
	}
	share(t, sfu, channelID, sharer, "screen-being-shared")

	if err := sfu.SetScreenActive(sharer, true); err != ErrNotAllowed {
		t.Fatalf("sharing without the permission = %v, want %v", err, ErrNotAllowed)
	}
	if err := sfu.SetMayStream(sharer, true); err != nil {
		t.Fatalf("grant stream: %v", err)
	}
	if err := sfu.SetScreenActive(sharer, true); err != nil {
		t.Errorf("granting stream back left the member unable to share: %v", err)
	}
}

func share(t *testing.T, sfu *SFU, channelID, sharer uuid.UUID, streamID string) {
	t.Helper()

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		TrackName(SourceScreen, publicOf(sharer), 1), streamID)
	if err != nil {
		t.Fatalf("create screen track: %v", err)
	}

	sfu.mu.Lock()
	defer sfu.mu.Unlock()

	r := sfu.rooms[channelID]
	p := r.peers[sharer]
	p.screenTrack = track
	p.owned[track.ID()] = true
	r.tracks[track.ID()] = track
	r.screens[sharer] = streamID
}

func TestLeavingWhileSharingTakesTheScreenDown(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, sharer, viewer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, sharer, publicOf(sharer), true); err != nil {
		t.Fatalf("join sharer: %v", err)
	}
	if err := sfu.Join(channelID, viewer, publicOf(viewer), true); err != nil {
		t.Fatalf("join viewer: %v", err)
	}

	const streamID = "screen-being-shared"
	sfu.mu.Lock()
	sfu.rooms[channelID].screens[sharer] = streamID
	sfu.mu.Unlock()

	sfu.Leave(sharer)

	if !signaler.sawScreen(screenChange{userID: publicOf(sharer), streamID: streamID, active: false}) {
		t.Error("somebody left mid-share and nobody was told the screen went away, so the tile and " +
			"its owner linger for every viewer")
	}
}

func TestDroppingAShareOnlyDropsThatPersonsScreen(t *testing.T) {
	sharer, other := uuid.New(), uuid.New()
	r := &room{layers: make(map[string]layer)}
	p := &peer{ignored: map[events.UserID]bool{publicOf(sharer): true}, sizes: map[events.UserID]string{}}

	cases := []struct {
		track string
		want  bool
		why   string
	}{
		{TrackName(SourceScreen, publicOf(sharer), 1), false, "the screen that was dropped is still sent"},
		{TrackName(SourceScreen, publicOf(other), 2), true, "dropping one screen took another with it"},
		{TrackName(SourceMicrophone, publicOf(sharer), 3), true,
			"dropping a screen silenced the person sharing it, so closing a tile leaves you unable to hear them"},
		{TrackName(SourceScreenAudio, publicOf(sharer), 4), true,
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
	p := &peer{ignored: map[events.UserID]bool{}, sizes: map[events.UserID]string{}}

	if !p.wants(r, TrackName(SourceScreen, publicOf(uuid.New()), 1)) {
		t.Error("a viewer who dropped nothing was refused a screen")
	}
}

func TestWatchingSomebodyWhileNotInACallFails(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.SetWatching(uuid.New(), publicOf(uuid.New()), false, ""); err != ErrNotConnected {
		t.Errorf("SetWatching error = %v, want %v", err, ErrNotConnected)
	}
}

func TestDroppingAndResumingAShareIsRemembered(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer, sharer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, publicOf(viewer), true); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.SetWatching(viewer, publicOf(sharer), false, ""); err != nil {
		t.Fatalf("stop watching: %v", err)
	}

	sfu.mu.Lock()
	dropped := sfu.rooms[channelID].peers[viewer].ignored[publicOf(sharer)]
	sfu.mu.Unlock()
	if !dropped {
		t.Fatal("a dropped share was not recorded, so the next renegotiation sends it again")
	}

	if err := sfu.SetWatching(viewer, publicOf(sharer), true, ""); err != nil {
		t.Fatalf("resume watching: %v", err)
	}

	sfu.mu.Lock()
	stillDropped := sfu.rooms[channelID].peers[viewer].ignored[publicOf(sharer)]
	sfu.mu.Unlock()
	if stillDropped {
		t.Error("resuming left the share dropped")
	}
}

func TestAViewerIsSentOneSizeOfAScreenAndNotTheOther(t *testing.T) {
	sharer, viewer := uuid.New(), uuid.New()
	full := TrackName(SourceScreen, publicOf(sharer), 1)
	half := TrackName(SourceScreen, publicOf(sharer), 2)

	r := &room{layers: map[string]layer{
		full: {owner: publicOf(sharer), rid: DefaultLayer},
		half: {owner: publicOf(sharer), rid: SmallerLayer},
	}}
	p := &peer{userID: viewer, ignored: map[events.UserID]bool{}, sizes: map[events.UserID]string{}}

	if !p.wants(r, full) || p.wants(r, half) {
		t.Fatal("a viewer who has asked for nothing is not being sent exactly one size; sending " +
			"both is the whole cost of simulcast with none of the benefit")
	}

	p.sizes[publicOf(sharer)] = SmallerLayer
	if p.wants(r, full) || !p.wants(r, half) {
		t.Error("asking for the smaller size did not switch which one is sent")
	}

	p.ignored[publicOf(sharer)] = true
	if p.wants(r, full) || p.wants(r, half) {
		t.Error("dropping a screen left one of its sizes still being sent")
	}
}

func TestAScreenWithNoLayersIsStillSent(t *testing.T) {
	sharer := uuid.New()
	only := TrackName(SourceScreen, publicOf(sharer), 1)

	r := &room{layers: map[string]layer{}}
	p := &peer{ignored: map[events.UserID]bool{}, sizes: map[events.UserID]string{}}

	if !p.wants(r, only) {
		t.Error("a screen published in one size was withheld; a browser that will not do " +
			"simulcast has to keep working exactly as it did")
	}
}

func TestAskingForASmallerScreenIsRemembered(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer, sharer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, publicOf(viewer), true); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.SetWatching(viewer, publicOf(sharer), true, SmallerLayer); err != nil {
		t.Fatalf("ask for the smaller size: %v", err)
	}

	sfu.mu.Lock()
	chosen := sfu.rooms[channelID].peers[viewer].sizeFor(publicOf(sharer))
	sfu.mu.Unlock()
	if chosen != SmallerLayer {
		t.Fatalf("chosen size = %q, want %q", chosen, SmallerLayer)
	}

	if err := sfu.SetWatching(viewer, publicOf(sharer), true, "enormous"); err != nil {
		t.Fatalf("ask for a size that does not exist: %v", err)
	}

	sfu.mu.Lock()
	after := sfu.rooms[channelID].peers[viewer].sizeFor(publicOf(sharer))
	sfu.mu.Unlock()
	if after != SmallerLayer {
		t.Errorf("a size the server does not publish overwrote a real choice: %q", after)
	}
}

func TestTakingAShareBackKeepsTheSizeYouChose(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer, sharer := uuid.New(), uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, publicOf(viewer), true); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.SetWatching(viewer, publicOf(sharer), true, SmallerLayer); err != nil {
		t.Fatalf("choose smaller: %v", err)
	}
	if err := sfu.SetWatching(viewer, publicOf(sharer), false, ""); err != nil {
		t.Fatalf("stop watching: %v", err)
	}
	if err := sfu.SetWatching(viewer, publicOf(sharer), true, ""); err != nil {
		t.Fatalf("watch again: %v", err)
	}

	sfu.mu.Lock()
	chosen := sfu.rooms[channelID].peers[viewer].sizeFor(publicOf(sharer))
	sfu.mu.Unlock()
	if chosen != SmallerLayer {
		t.Error("watching again reset somebody to the full size, so a viewer who chose smaller " +
			"because their connection could not take it is handed the large one the moment they " +
			"look away and back")
	}
}

func TestAMemberWhoMayNotStreamCannotPublishAScreen(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	sfu.AttachPublishSignaler(&recordingPublishSignaler{
		answers: make(chan webrtc.SessionDescription, 1),
	})

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, publicOf(userID), false); err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := sfu.PublishScreen(userID, webrtc.SessionDescription{}); err != ErrNotAllowed {
		t.Fatalf("PublishScreen error = %v, want %v; the video section used to be the refusal "+
			"and there is no video section any more, so this check is now the whole of it", err, ErrNotAllowed)
	}
}

func TestJoiningOffersOnlyAMicrophone(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	userID := uuid.New()
	if err := sfu.Join(uuid.New(), userID, publicOf(userID), true); err != nil {
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
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, publicOf(userID), true); err != nil {
		t.Fatalf("join: %v", err)
	}
	first := sfu.peerFor(userID)
	if first == nil {
		t.Fatal("joining left nobody in the room")
	}

	if err := sfu.Join(channelID, userID, publicOf(userID), true); err != nil {
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
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	signals := &recordingPublishSignaler{answers: make(chan webrtc.SessionDescription, 1)}
	sfu.AttachPublishSignaler(signals)

	userID := uuid.New()
	if err := sfu.Join(uuid.New(), userID, publicOf(userID), true); err != nil {
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

func TestATrackThatArrivesMidNegotiationIsStillOffered(t *testing.T) {
	signaler := newRecordingSignaler()
	sfu, err := New(signaler, nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, viewer := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, viewer, publicOf(viewer), true); err != nil {
		t.Fatalf("join: %v", err)
	}
	first := signaler.offerFor(t, viewer).sdp
	if got := audioSections(first.SDP); got != 1 {
		t.Fatalf("the opening offer has %d audio sections, want 1", got)
	}

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		TrackName(SourceScreenAudio, publicOf(uuid.New()), 1), "share")
	if err != nil {
		t.Fatalf("create track: %v", err)
	}

	sfu.mu.Lock()
	r := sfu.rooms[channelID]
	r.tracks[track.ID()] = track
	sfu.signalLocked(r)
	sfu.mu.Unlock()

	answerOffer(t, sfu, viewer, first)

	sfu.mu.Lock()
	sfu.signalLocked(r)
	sfu.mu.Unlock()

	latest := signaler.offerFor(t, viewer).sdp
	if got := audioSections(latest.SDP); got != 2 {
		t.Errorf("the viewer was offered %d audio sections, want 2; a track that appeared while "+
			"the viewer was still answering an earlier offer was attached to their connection and "+
			"then never offered, because the retry sees it among the senders and concludes there "+
			"is nothing left to do. Screen sound arrives one renegotiation after screen video, "+
			"which is exactly when this happens", got)
	}
}

func answerOffer(t *testing.T, sfu *SFU, userID uuid.UUID, offer webrtc.SessionDescription) {
	t.Helper()

	client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("client peer connection: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	if err := client.SetRemoteDescription(offer); err != nil {
		t.Fatalf("set remote: %v", err)
	}
	answer, err := client.CreateAnswer(nil)
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	if err := client.SetLocalDescription(answer); err != nil {
		t.Fatalf("set local: %v", err)
	}
	if err := sfu.Answer(userID, answer); err != nil {
		t.Fatalf("answer: %v", err)
	}
}

func publicOf(userID uuid.UUID) events.UserID {
	return events.UserID(strings.ToLower(
		base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(userID[:10])))
}
