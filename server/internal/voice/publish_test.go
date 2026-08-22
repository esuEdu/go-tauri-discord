package voice

import (
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
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	signals := &recordingPublishSignaler{answers: make(chan webrtc.SessionDescription, 1)}
	sfu.AttachPublishSignaler(signals)

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

	userID := uuid.New()
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
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if err := sfu.PublishScreen(uuid.New(), webrtc.SessionDescription{}); err != ErrNotConnected {
		t.Errorf("PublishScreen error = %v, want %v", err, ErrNotConnected)
	}
}

func TestLayersOfSomebodyNotPublishingAreNothing(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if got := sfu.PublishedLayers(uuid.New()); got != nil {
		t.Errorf("PublishedLayers of a stranger = %v, want nil", got)
	}
}
