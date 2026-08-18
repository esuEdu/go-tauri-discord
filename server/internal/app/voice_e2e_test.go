//go:build e2e

package app_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type voiceClient struct {
	t      *testing.T
	sock   *socket
	pc     *webrtc.PeerConnection
	track  *webrtc.TrackLocalStaticSample
	remote chan *webrtc.TrackRemote
	done   chan struct{}

	mu        sync.Mutex
	screen    *webrtc.TrackLocalStaticSample
	screenMid string
}

func newVoiceClient(t *testing.T, h *harness) *voiceClient {
	t.Helper()

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("create peer connection: %v", err)
	}

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "vocalis")
	if err != nil {
		t.Fatalf("create track: %v", err)
	}
	if _, err := pc.AddTrack(track); err != nil {
		t.Fatalf("add track: %v", err)
	}

	c := &voiceClient{
		t:      t,
		sock:   h.dial(),
		pc:     pc,
		track:  track,
		remote: make(chan *webrtc.TrackRemote, 4),
		done:   make(chan struct{}),
	}

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		select {
		case c.remote <- remote:
		default:
		}
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		c.sock.write(events.Frame{
			Op: events.OpVoiceCandidate,
			D: mustJSON(t, events.ICECandidate{
				Candidate:        init.Candidate,
				SDPMid:           init.SDPMid,
				SDPMLineIndex:    init.SDPMLineIndex,
				UsernameFragment: init.UsernameFragment,
			}),
		})
	})

	c.sock.identify(h.token)
	t.Cleanup(func() {
		close(c.done)
		pc.Close()
	})
	return c
}

func (c *voiceClient) pump() {
	go func() {
		for {
			select {
			case <-c.done:
				return
			default:
			}

			frame := c.sock.readAny()
			if frame == nil {
				return
			}

			switch frame.Op {
			case events.OpVoiceOffer:
				var sdp events.SessionDescription
				if json.Unmarshal(frame.D, &sdp) != nil {
					continue
				}
				if err := c.pc.SetRemoteDescription(webrtc.SessionDescription{
					Type: webrtc.NewSDPType(sdp.Type), SDP: sdp.SDP,
				}); err != nil {
					continue
				}
				c.rememberScreenMid(sdp.ScreenMid)
				c.attachScreen(sdp.ScreenMid)
				answer, err := c.pc.CreateAnswer(nil)
				if err != nil {
					continue
				}
				if err := c.pc.SetLocalDescription(answer); err != nil {
					continue
				}
				c.sock.write(events.Frame{
					Op: events.OpVoiceAnswer,
					D:  mustJSON(c.t, events.SessionDescription{Type: answer.Type.String(), SDP: answer.SDP}),
				})

			case events.OpVoiceCandidate:
				var candidate events.ICECandidate
				if json.Unmarshal(frame.D, &candidate) != nil {
					continue
				}
				_ = c.pc.AddICECandidate(webrtc.ICECandidateInit{
					Candidate:        candidate.Candidate,
					SDPMid:           candidate.SDPMid,
					SDPMLineIndex:    candidate.SDPMLineIndex,
					UsernameFragment: candidate.UsernameFragment,
				})
			}
		}
	}()
}

func (c *voiceClient) join(channelID uuid.UUID) {
	c.sock.write(events.Frame{
		Op: events.OpVoiceState,
		D:  mustJSON(c.t, events.VoiceStateRequest{ChannelID: &channelID}),
	})
}

func (c *voiceClient) streamSilence() {
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		silence := make([]byte, 80)
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				_ = c.track.WriteSample(media.Sample{Data: silence, Duration: 20 * time.Millisecond})
			}
		}
	}()
}

func (c *voiceClient) rememberScreenMid(mid *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if mid != nil {
		c.screenMid = *mid
	}
}

func (c *voiceClient) reservedMid() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.screenMid
}

func (c *voiceClient) midOf(track *webrtc.TrackRemote) string {
	for _, transceiver := range c.pc.GetTransceivers() {
		if receiver := transceiver.Receiver(); receiver != nil {
			for _, candidate := range receiver.Tracks() {
				if candidate == track {
					return transceiver.Mid()
				}
			}
		}
	}
	return ""
}

func (c *voiceClient) attachScreen(mid *string) {
	c.mu.Lock()
	track := c.screen
	c.mu.Unlock()

	if track == nil || mid == nil {
		return
	}
	for _, transceiver := range c.pc.GetTransceivers() {
		if transceiver.Mid() != *mid {
			continue
		}
		sender := transceiver.Sender()
		if sender == nil {
			if _, err := c.pc.AddTrack(track); err != nil {
				c.t.Logf("attach screen track: %v", err)
			}
			return
		}
		if sender.Track() != track {
			if err := sender.ReplaceTrack(track); err != nil {
				c.t.Logf("replace screen track: %v", err)
			}
		}
		return
	}
}

func (c *voiceClient) share() {
	c.t.Helper()

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "screen", "vocalis-screen")
	if err != nil {
		c.t.Fatalf("create screen track: %v", err)
	}

	c.mu.Lock()
	c.screen = track
	c.mu.Unlock()

	c.sock.write(events.Frame{Op: events.OpVoiceResync})

	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()
		frame := make([]byte, 128)
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				_ = track.WriteSample(media.Sample{Data: frame, Duration: 33 * time.Millisecond})
			}
		}
	}()
}

func (c *voiceClient) stopSharing() {
	c.t.Helper()

	c.mu.Lock()
	mid := c.screenMid
	c.screen = nil
	c.mu.Unlock()

	for _, transceiver := range c.pc.GetTransceivers() {
		if transceiver.Mid() != mid {
			continue
		}
		if sender := transceiver.Sender(); sender != nil {
			if err := sender.ReplaceTrack(nil); err != nil {
				c.t.Logf("clear screen track: %v", err)
			}
		}
	}
	c.sock.write(events.Frame{Op: events.OpVoiceResync})
}

func TestVoiceMediaFlowsBetweenTwoMembers(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Voice")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	alice := newVoiceClient(t, owner)
	bob := newVoiceClient(t, friend)
	alice.pump()
	bob.pump()

	alice.join(voiceChannel)
	alice.streamSilence()
	time.Sleep(500 * time.Millisecond)

	bob.join(voiceChannel)
	bob.streamSilence()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	select {
	case remote := <-bob.remote:
		if remote.Kind() != webrtc.RTPCodecTypeAudio {
			t.Fatalf("bob received a %s track, want audio", remote.Kind())
		}
		buf := make([]byte, 1500)
		if err := remote.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := remote.Read(buf); err != nil {
			t.Fatalf("no RTP arrived on the forwarded track: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("bob never received alice's audio track through the SFU")
	}
}

func TestVoiceStateBroadcastsToTheGuild(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Voice States")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	watcher := owner.dial()
	watcher.identify(owner.token)

	speaker := newVoiceClient(t, owner)
	speaker.pump()
	speaker.join(voiceChannel)

	frame := watcher.readEvent(events.EventVoiceStateUpdate)
	var state events.VoiceStateUpdate
	decode(t, frame.D, &state)

	if state.ChannelID == nil || *state.ChannelID != voiceChannel {
		t.Fatalf("voice state channel = %v, want %s", state.ChannelID, voiceChannel)
	}
	if state.GuildID != guild.ID {
		t.Errorf("voice state guild = %s, want %s", state.GuildID, guild.ID)
	}
}

func TestVoiceRejectsTextChannels(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("No Voice In Text")
	textChannel, _ := h.textAndVoice(guild.ID)

	watcher := h.dial()
	watcher.identify(h.token)

	client := newVoiceClient(t, h)
	client.pump()
	client.join(textChannel)

	quiet := watcher.quietFor(3*time.Second, func(f events.Frame) bool {
		return f.T == events.EventVoiceStateUpdate
	})
	if !quiet {
		t.Error("joining a text channel produced a voice state update")
	}
}
