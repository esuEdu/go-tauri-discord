//go:build e2e

package app_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/esuEdu/go-tauri-discord/internal/voice"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type voiceClient struct {
	t      *testing.T
	sock   *socket
	pc     *webrtc.PeerConnection
	track  *webrtc.TrackLocalStaticSample
	remote chan *webrtc.TrackRemote
	done   chan struct{}

	keyframes chan struct{}

	mu             sync.Mutex
	screen         *webrtc.TrackLocalStaticSample
	screenSound    *webrtc.TrackLocalStaticSample
	screenMid      string
	screenSoundMid string
	screenStop     chan struct{}
	publishPC      *webrtc.PeerConnection
	publishTrack   *webrtc.TrackLocalStaticSample
	writing        bool
	writingSound   bool
}

func (c *voiceClient) watchKeyframeRequests(sender *webrtc.RTPSender) {
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			return
		}
		for _, packet := range packets {
			if _, ok := packet.(*rtcp.PictureLossIndication); !ok {
				continue
			}
			select {
			case c.keyframes <- struct{}{}:
			default:
			}
		}
	}
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
		t:         t,
		sock:      h.dial(),
		pc:        pc,
		track:     track,
		remote:    make(chan *webrtc.TrackRemote, 4),
		keyframes: make(chan struct{}, 4),
		done:      make(chan struct{}),
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

			case events.OpScreenAnswer:
				var sdp events.ScreenPublish
				if json.Unmarshal(frame.D, &sdp) != nil {
					continue
				}
				c.mu.Lock()
				pc := c.publishPC
				c.mu.Unlock()
				if pc != nil {
					_ = pc.SetRemoteDescription(webrtc.SessionDescription{
						Type: webrtc.SDPTypeAnswer, SDP: sdp.SDP,
					})
				}

			case events.OpScreenIce:
				var candidate events.ICECandidate
				if json.Unmarshal(frame.D, &candidate) != nil {
					continue
				}
				c.mu.Lock()
				pc := c.publishPC
				c.mu.Unlock()
				if pc != nil {
					_ = pc.AddICECandidate(webrtc.ICECandidateInit{
						Candidate:        candidate.Candidate,
						SDPMid:           candidate.SDPMid,
						SDPMLineIndex:    candidate.SDPMLineIndex,
						UsernameFragment: candidate.UsernameFragment,
					})
				}

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

func (c *voiceClient) drainKeyframes() {
	for {
		select {
		case <-c.keyframes:
		default:
			return
		}
	}
}

func (c *voiceClient) awaitScreen(timeout time.Duration) *webrtc.TrackRemote {
	c.t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case remote := <-c.remote:
			if remote.Kind() == webrtc.RTPCodecTypeVideo {
				return remote
			}
		case <-deadline:
			c.t.Fatal("no screen track arrived through the SFU")
			return nil
		}
	}
}

func (c *voiceClient) awaitTrack(source voice.Source, owner uuid.UUID, timeout time.Duration) *webrtc.TrackRemote {
	c.t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case remote := <-c.remote:
			got, from, ok := voice.ParseTrackName(remote.StreamID())
			if ok && got == source && from == owner {
				return remote
			}
		case <-deadline:
			c.t.Fatalf("no %s track owned by %s arrived through the SFU", source, owner)
			return nil
		}
	}
}

func (c *voiceClient) askKeyframe(remote *webrtc.TrackRemote) {
	c.t.Helper()

	if err := c.pc.WriteRTCP([]rtcp.Packet{
		&rtcp.PictureLossIndication{MediaSSRC: uint32(remote.SSRC())},
	}); err != nil {
		c.t.Fatalf("send picture loss indication: %v", err)
	}
}

func (c *voiceClient) rememberScreenMids(video, sound *string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if video != nil {
		c.screenMid = *video
	}
	if sound != nil {
		c.screenSoundMid = *sound
	}
}

func (c *voiceClient) reservedMid() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.screenMid
}

func (c *voiceClient) reservedSoundMid() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.screenSoundMid
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

func (c *voiceClient) attachAt(mid string, track *webrtc.TrackLocalStaticSample) (*webrtc.RTPSender, bool) {
	for _, transceiver := range c.pc.GetTransceivers() {
		if transceiver.Mid() != mid {
			continue
		}
		sender := transceiver.Sender()
		if sender == nil {
			added, err := c.pc.AddTrack(track)
			if err != nil {
				c.t.Logf("attach track on mid %s: %v", mid, err)
				return nil, false
			}
			return added, true
		}
		if sender.Track() != track {
			if err := sender.ReplaceTrack(track); err != nil {
				c.t.Logf("replace track on mid %s: %v", mid, err)
			}
		}
		return sender, false
	}
	return nil, false
}

func (c *voiceClient) attachScreen(mid *string) {
	c.mu.Lock()
	track := c.screen
	c.mu.Unlock()

	if track == nil || mid == nil {
		return
	}
	sender, fresh := c.attachAt(*mid, track)
	if sender == nil {
		return
	}
	if fresh {
		go c.watchKeyframeRequests(sender)
	}
	c.startWriting(track)
}

func (c *voiceClient) attachScreenSound(mid *string) {
	c.mu.Lock()
	track := c.screenSound
	c.mu.Unlock()

	if track == nil || mid == nil {
		return
	}
	if sender, _ := c.attachAt(*mid, track); sender == nil {
		return
	}
	c.startWritingSound(track)
}

func (c *voiceClient) share() {
	c.t.Helper()
	c.startSharing(false)
}

func (c *voiceClient) shareWithSound() {
	c.t.Helper()
	c.startSharing(true)
}

func (c *voiceClient) startSharing(withSound bool) {
	c.t.Helper()
	c.publishScreen(withSound)
	c.announceScreen(true)
}

func (c *voiceClient) startWriting(track *webrtc.TrackLocalStaticSample) {
	c.mu.Lock()
	if c.writing || c.screenStop == nil {
		c.mu.Unlock()
		return
	}
	c.writing = true
	stop := c.screenStop
	c.mu.Unlock()

	go c.writeScreen(track, stop)
}

func (c *voiceClient) startWritingSound(track *webrtc.TrackLocalStaticSample) {
	c.mu.Lock()
	if c.writingSound || c.screenStop == nil {
		c.mu.Unlock()
		return
	}
	c.writingSound = true
	stop := c.screenStop
	c.mu.Unlock()

	go c.writeScreenSound(track, stop)
}

func (c *voiceClient) writeScreenSound(track *webrtc.TrackLocalStaticSample, stop chan struct{}) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	sound := make([]byte, 80)
	for {
		select {
		case <-c.done:
			return
		case <-stop:
			return
		case <-ticker.C:
			_ = track.WriteSample(media.Sample{Data: sound, Duration: 20 * time.Millisecond})
		}
	}
}

func (c *voiceClient) writeScreen(track *webrtc.TrackLocalStaticSample, stop chan struct{}) {
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()
	frame := make([]byte, 128)
	for {
		select {
		case <-c.done:
			return
		case <-stop:
			return
		case <-ticker.C:
			_ = track.WriteSample(media.Sample{Data: frame, Duration: 33 * time.Millisecond})
		}
	}
}

func (c *voiceClient) announceScreen(active bool) {
	c.sock.write(events.Frame{
		Op: events.OpVoiceScreen,
		D:  mustJSON(c.t, events.VoiceScreenRequest{Active: active}),
	})
}

func (c *voiceClient) stopSharingQuietly() {
	c.t.Helper()

	c.mu.Lock()
	stop := c.screenStop
	c.screenStop = nil
	c.writing = false
	c.writingSound = false
	c.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	c.announceScreen(false)
}

func (c *voiceClient) resumeSharing() {
	c.t.Helper()

	c.mu.Lock()
	track := c.screen
	sound := c.screenSound
	c.screenStop = make(chan struct{})
	c.writing = false
	c.writingSound = false
	c.mu.Unlock()

	if track == nil {
		c.t.Fatal("resumed a share that was never started")
	}
	c.announceScreen(true)
	c.startWriting(track)
	if sound != nil {
		c.startWritingSound(sound)
	}
}

func (c *voiceClient) stopSharing() {
	c.t.Helper()

	c.mu.Lock()
	mids := map[string]bool{c.screenMid: true, c.screenSoundMid: true}
	delete(mids, "")
	stop := c.screenStop
	c.screen = nil
	c.screenSound = nil
	c.screenStop = nil
	c.writing = false
	c.writingSound = false
	c.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	for _, transceiver := range c.pc.GetTransceivers() {
		if !mids[transceiver.Mid()] {
			continue
		}
		if sender := transceiver.Sender(); sender != nil {
			if err := sender.ReplaceTrack(nil); err != nil {
				c.t.Logf("clear screen track: %v", err)
			}
		}
	}
	c.announceScreen(false)
}

func TestVoiceMediaFlowsBetweenTwoMembers(t *testing.T) {
	owner := newHarness(t)
	aliceID, _ := owner.registerUser()
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
		source, owner, ok := voice.ParseTrackName(remote.StreamID())
		if !ok || source != voice.SourceMicrophone || owner != aliceID {
			t.Fatalf("forwarded stream %q does not name alice (%s) as the speaker",
				remote.StreamID(), aliceID)
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

func (c *voiceClient) setMuted(muted bool) {
	c.sock.write(events.Frame{
		Op: events.OpVoiceMute,
		D:  mustJSON(c.t, events.VoiceMuteRequest{SelfMute: muted}),
	})
}

func awaitVoiceState(t *testing.T, s *socket, userID uuid.UUID) events.VoiceStateUpdate {
	t.Helper()
	for range 10 {
		var state events.VoiceStateUpdate
		decode(t, s.readEvent(events.EventVoiceStateUpdate).D, &state)
		if state.UserID == userID {
			return state
		}
	}
	t.Fatalf("no voice state about %s arrived", userID)
	return events.VoiceStateUpdate{}
}

func TestMutingIsAnnouncedToTheChannel(t *testing.T) {
	owner := newHarness(t)
	speakerID, _ := owner.registerUser()
	guild := owner.createGuild("Mutes")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	watcher := friend.dial()
	watcher.identify(friend.token)

	speaker := newVoiceClient(t, owner)
	speaker.pump()
	speaker.join(voiceChannel)
	speaker.streamSilence()

	if state := awaitVoiceState(t, watcher, speakerID); state.SelfMute {
		t.Fatal("a member arrived already muted")
	}

	speaker.setMuted(true)

	state := awaitVoiceState(t, watcher, speakerID)
	if !state.SelfMute {
		t.Fatal("muting reached nobody, so a muted member is indistinguishable from a quiet one")
	}
	if state.ChannelID == nil || *state.ChannelID != voiceChannel {
		t.Errorf("mute update named channel %v, want %s", state.ChannelID, voiceChannel)
	}
}

func TestSomebodyJoiningLaterIsToldWhoIsMuted(t *testing.T) {
	owner := newHarness(t)
	speakerID, _ := owner.registerUser()
	guild := owner.createGuild("Late Joiners")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	speaker := newVoiceClient(t, owner)
	speaker.pump()
	speaker.join(voiceChannel)
	speaker.streamSilence()
	time.Sleep(500 * time.Millisecond)

	speaker.setMuted(true)
	time.Sleep(500 * time.Millisecond)

	latecomer := friend.dial()
	latecomer.identify(friend.token)
	latecomer.write(events.Frame{
		Op: events.OpVoiceState,
		D:  mustJSON(t, events.VoiceStateRequest{ChannelID: &voiceChannel}),
	})

	state := awaitVoiceState(t, latecomer, speakerID)
	if !state.SelfMute {
		t.Fatal("the snapshot sent on joining reported a muted member as live; mute would only " +
			"become visible the next time they happened to toggle it")
	}
}

func (c *voiceClient) publishScreen(withSound bool) {
	c.t.Helper()

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		c.t.Fatalf("publish peer connection: %v", err)
	}
	c.t.Cleanup(func() { pc.Close() })

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "screen", "share")
	if err != nil {
		c.t.Fatalf("publish track: %v", err)
	}

	sender, err := pc.AddTrack(track)
	if err != nil {
		c.t.Fatalf("add publish track: %v", err)
	}
	go c.watchKeyframeRequests(sender)

	if withSound {
		sound, err := webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "sound", "share")
		if err != nil {
			c.t.Fatalf("publish sound track: %v", err)
		}
		if _, err := pc.AddTrack(sound); err != nil {
			c.t.Fatalf("add publish sound: %v", err)
		}
		c.screenSound = sound
	}

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		c.sock.write(events.Frame{
			Op: events.OpScreenIce,
			D: mustJSON(c.t, events.ICECandidate{
				Candidate: init.Candidate, SDPMid: init.SDPMid,
				SDPMLineIndex: init.SDPMLineIndex, UsernameFragment: init.UsernameFragment,
			}),
		})
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		c.t.Fatalf("publish offer: %v", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		c.t.Fatalf("publish set local: %v", err)
	}

	c.mu.Lock()
	c.publishPC = pc
	c.publishTrack = track
	c.screen = track
	c.screenStop = make(chan struct{})
	stop := c.screenStop
	sound := c.screenSound
	c.mu.Unlock()

	c.sock.write(events.Frame{
		Op: events.OpScreenPublish,
		D:  mustJSON(c.t, events.ScreenPublish{SDP: offer.SDP}),
	})

	go c.writeScreen(track, stop)
	if sound != nil {
		go c.writeScreenSound(sound, stop)
	}
}
