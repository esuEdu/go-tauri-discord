package voice

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type recordingPublishSignaler struct {
	answers chan webrtc.SessionDescription
}

func (r *recordingPublishSignaler) ScreenAnswer(_ uuid.UUID, sdp webrtc.SessionDescription) {
	select {
	case r.answers <- sdp:
	default:
	}
}

func (r *recordingPublishSignaler) ScreenCandidate(uuid.UUID, webrtc.ICECandidateInit) {}

func TestTheServerAcceptsAScreenPublishedInSeveralSizes(t *testing.T) {
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

	client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("client peer connection: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "screen", "share",
		webrtc.WithRTPStreamID("full"))
	if err != nil {
		t.Fatalf("create track: %v", err)
	}
	if _, err := client.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	}); err != nil {
		t.Fatalf("add transceiver: %v", err)
	}

	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := client.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local: %v", err)
	}

	if err := sfu.PublishScreen(userID, offer); err != nil {
		t.Fatalf("publish screen: %v", err)
	}

	select {
	case answer := <-signals.answers:
		if answer.Type != webrtc.SDPTypeAnswer {
			t.Fatalf("server replied with %s, want an answer", answer.Type)
		}
		if err := client.SetRemoteDescription(answer); err != nil {
			t.Fatalf("the server's answer was not usable: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server never answered a screen offered by the client; publishing on a " +
			"connection the client offers is what simulcast needs and this is the whole of it")
	}
}

func TestPublishingWithoutSomewhereToReplyIsRefused(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.PublishScreen(uuid.New(), webrtc.SessionDescription{}); err != ErrNotConnected {
		t.Errorf("PublishScreen error = %v, want %v", err, ErrNotConnected)
	}
}

func TestLayersOfSomebodyNotPublishingAreNothing(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if got := sfu.PublishedLayers(uuid.New()); got != nil {
		t.Errorf("PublishedLayers of a stranger = %v, want nil", got)
	}
}

func publishOneScreen(t *testing.T, sfu *SFU, userID uuid.UUID) {
	t.Helper()

	client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("client peer connection: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "screen", "share")
	if err != nil {
		t.Fatalf("create track: %v", err)
	}
	if _, err := client.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	}); err != nil {
		t.Fatalf("add transceiver: %v", err)
	}

	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	if err := client.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local: %v", err)
	}
	if err := sfu.PublishScreen(userID, offer); err != nil {
		t.Fatalf("publish screen: %v", err)
	}
}

func TestPublishingWhileOthersJoinKeepsTheirEstimatesApart(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil, Network{})
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)
	sfu.AttachPublishSignaler(&recordingPublishSignaler{
		answers: make(chan webrtc.SessionDescription, 1),
	})

	channelID, sharer := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, sharer, publicOf(sharer), true); err != nil {
		t.Fatalf("join sharer: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 8 {
			publishOneScreen(t, sfu, sharer)
		}
	}()
	go func() {
		defer wg.Done()
		for range 8 {
			listener := uuid.New()
			if err := sfu.Join(channelID, listener, publicOf(listener), false); err != nil {
				t.Errorf("join listener: %v", err)
			}
		}
	}()
	wg.Wait()

	for _, e := range sfu.Estimates(channelID) {
		if e.Bits <= 0 {
			t.Errorf("%s has no bandwidth estimate of its own: the estimator a new connection "+
				"reports is picked up from one shared field, and publishing a screen builds a "+
				"connection without taking the lock that field is guarded by, so a listener and "+
				"a publisher arriving together can take each other's estimate or none at all",
				e.UserID)
		}
	}
}
