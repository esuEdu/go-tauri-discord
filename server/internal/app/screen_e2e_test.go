//go:build e2e

package app_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"

	"github.com/esuEdu/go-tauri-discord/internal/voice"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func readScreen(t *testing.T, remote *webrtc.TrackRemote) {
	t.Helper()

	buf := make([]byte, 1500)
	if err := remote.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := remote.Read(buf); err != nil {
		t.Fatalf("no RTP arrived on the forwarded screen track: %v", err)
	}
}

func TestScreenVideoReachesAnotherMember(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Screens")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	viewer := newVoiceClient(t, friend)
	presenter.pump()
	viewer.pump()

	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)

	viewer.join(voiceChannel)
	viewer.streamSilence()
	time.Sleep(500 * time.Millisecond)

	presenter.share()

	remote := viewer.awaitScreen(30 * time.Second)
	readScreen(t, remote)

	if mid := viewer.midOf(remote); mid == viewer.reservedMid() {
		t.Fatalf("the presenter's screen arrived on mid %q, which is the viewer's own "+
			"upload slot; a viewer that sets a direction on that slot silences the share", mid)
	}
	if id := remote.StreamID(); id == "" || id == "-" {
		t.Fatalf("the forwarded screen carried stream id %q, so a browser reports no "+
			"stream on the track event and the viewer drops it before rendering", id)
	}
}

func TestScreenSoundReachesAnotherMember(t *testing.T) {
	owner := newHarness(t)
	presenterID, _ := owner.registerUser()
	guild := owner.createGuild("Screen Sound")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	viewer := newVoiceClient(t, friend)
	presenter.pump()
	viewer.pump()

	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)

	viewer.join(voiceChannel)
	viewer.streamSilence()
	time.Sleep(500 * time.Millisecond)

	presenter.shareWithSound()

	remote := viewer.awaitTrack(voice.SourceScreenAudio, presenterID, 30*time.Second)
	readScreen(t, remote)

	if remote.Kind() != webrtc.RTPCodecTypeAudio {
		t.Fatalf("the presenter's screen sound arrived as %s", remote.Kind())
	}
	if mid := viewer.midOf(remote); mid == viewer.reservedSoundMid() {
		t.Fatalf("the sound of the presenter's screen arrived on mid %q, which is the viewer's "+
			"own upload slot for screen sound", mid)
	}
}

func TestScreenShareAnnouncesWhoIsSharing(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Screen States")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	watcher := owner.dial()
	ready := watcher.identify(owner.token)

	presenter := newVoiceClient(t, owner)
	presenter.pump()
	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)
	presenter.share()

	frame := watcher.readEvent(events.EventVoiceScreenUpdate)
	var update events.VoiceScreenUpdate
	decode(t, frame.D, &update)

	if !update.Active {
		t.Error("screen update reported inactive while the presenter was sharing")
	}
	if update.ChannelID != voiceChannel {
		t.Errorf("screen update channel = %s, want %s", update.ChannelID, voiceChannel)
	}
	if update.GuildID != guild.ID {
		t.Errorf("screen update guild = %s, want %s", update.GuildID, guild.ID)
	}
	if update.UserID != ready.User.ID {
		t.Errorf("screen update user = %s, want %s", update.UserID, ready.User.ID)
	}
	if update.StreamID == "" {
		t.Error("screen update carried no stream id")
	}
}

func TestScreenShareRetractionIsAnnounced(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Screen Stops")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	watcher := owner.dial()
	watcher.identify(owner.token)

	presenter := newVoiceClient(t, owner)
	presenter.pump()
	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)
	presenter.share()

	started := watcher.readEvent(events.EventVoiceScreenUpdate)
	var begin events.VoiceScreenUpdate
	decode(t, started.D, &begin)
	if !begin.Active {
		t.Fatal("the first screen update was not a start")
	}

	presenter.stopSharing()

	stopped := watcher.readEvent(events.EventVoiceScreenUpdate)
	var end events.VoiceScreenUpdate
	decode(t, stopped.D, &end)
	if end.Active {
		t.Error("stopping a share never produced an inactive update, so viewers keep a frozen tile")
	}
	if end.StreamID != begin.StreamID {
		t.Errorf("retraction named stream %q, want %q", end.StreamID, begin.StreamID)
	}
}

func TestScreenRetractionDoesNotNeedTheTrackToDie(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Quiet Stops")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	watcher := owner.dial()
	watcher.identify(owner.token)

	presenter := newVoiceClient(t, owner)
	presenter.pump()
	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)
	presenter.share()

	started := watcher.readEvent(events.EventVoiceScreenUpdate)
	var begin events.VoiceScreenUpdate
	decode(t, started.D, &begin)
	if !begin.Active {
		t.Fatal("the first screen update was not a start")
	}

	presenter.stopSharingQuietly()

	retracted := !watcher.quietFor(10*time.Second, func(frame events.Frame) bool {
		if frame.T != events.EventVoiceScreenUpdate {
			return false
		}
		var end events.VoiceScreenUpdate
		decode(t, frame.D, &end)
		return !end.Active && end.StreamID == begin.StreamID
	})
	if !retracted {
		t.Fatal("a share that went quiet without its track dying was never retracted; a browser " +
			"stops sending on the same SSRC rather than ending the track, so its viewers would " +
			"keep the last frame on screen for the rest of the call")
	}
}

func TestResharingAfterAQuietStopReachesViewersAgain(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Reshares")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	viewer := newVoiceClient(t, friend)
	presenter.pump()
	viewer.pump()

	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)

	viewer.join(voiceChannel)
	viewer.streamSilence()
	time.Sleep(500 * time.Millisecond)

	presenter.share()
	first := viewer.awaitScreen(30 * time.Second)
	readScreen(t, first)

	presenter.stopSharingQuietly()
	time.Sleep(time.Second)
	presenter.resumeSharing()

	again := viewer.awaitScreen(30 * time.Second)
	readScreen(t, again)
}

func TestScreenSubscriberTriggersAKeyframeRequest(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Keyframes")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	presenter.pump()
	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)
	presenter.share()

	time.Sleep(time.Second)
	presenter.drainKeyframes()

	viewer := newVoiceClient(t, friend)
	viewer.pump()
	viewer.join(voiceChannel)
	viewer.streamSilence()

	select {
	case <-presenter.keyframes:
	case <-time.After(10 * time.Second):
		t.Fatal("the presenter was never asked for a keyframe after a viewer subscribed, so the " +
			"viewer waits for a natural keyframe while showing black")
	}
}

func TestIdleShareIsNotAskedForKeyframes(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Idle Keyframes")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	presenter := newVoiceClient(t, owner)
	presenter.pump()
	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)
	presenter.share()

	time.Sleep(2 * time.Second)
	presenter.drainKeyframes()

	select {
	case <-presenter.keyframes:
		t.Fatal("a share with nobody watching was asked for a keyframe anyway; blind keyframes " +
			"spend the bitrate that would otherwise buy detail on a static screen")
	case <-time.After(5 * time.Second):
	}
}

func TestViewerKeyframeRequestReachesThePublisher(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Relayed Keyframes")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	viewer := newVoiceClient(t, friend)
	presenter.pump()
	viewer.pump()

	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)

	viewer.join(voiceChannel)
	viewer.streamSilence()
	time.Sleep(500 * time.Millisecond)

	presenter.share()
	remote := viewer.awaitScreen(30 * time.Second)

	time.Sleep(2 * time.Second)
	presenter.drainKeyframes()

	viewer.askKeyframe(remote)

	select {
	case <-presenter.keyframes:
	case <-time.After(10 * time.Second):
		t.Fatal("a viewer whose decoder asked for a keyframe was never relayed to the publisher, " +
			"so a tile knocked out by packet loss stays black for the rest of the call")
	}
}

func (c *voiceClient) watch(sharer uuid.UUID, watching bool) {
	c.sock.write(events.Frame{
		Op: events.OpVoiceWatch,
		D:  mustJSON(c.t, events.VoiceWatchRequest{UserID: sharer, Watching: watching}),
	})
}

func TestAViewerCanDropAShareAndTakeItBack(t *testing.T) {
	owner := newHarness(t)
	presenterID, _ := owner.registerUser()
	guild := owner.createGuild("Dropped Shares")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	viewer := newVoiceClient(t, friend)
	presenter.pump()
	viewer.pump()

	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)

	viewer.join(voiceChannel)
	viewer.streamSilence()
	time.Sleep(500 * time.Millisecond)

	presenter.share()
	remote := viewer.awaitScreen(30 * time.Second)
	readScreen(t, remote)

	viewer.watch(presenterID, false)

	if err := remote.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1500)
	for {
		if _, _, err := remote.Read(buf); err != nil {
			break
		}
	}

	viewer.watch(presenterID, true)

	again := viewer.awaitScreen(30 * time.Second)
	readScreen(t, again)
}

func TestAScreenPublishedOnItsOwnConnectionReachesAViewer(t *testing.T) {
	owner := newHarness(t)
	presenterID, _ := owner.registerUser()
	guild := owner.createGuild("Own Connection")
	_, voiceChannel := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo("POST", "/api/v1/invites/"+invite.Code, 200, nil, nil)

	presenter := newVoiceClient(t, owner)
	viewer := newVoiceClient(t, friend)
	presenter.pump()
	viewer.pump()

	presenter.join(voiceChannel)
	presenter.streamSilence()
	time.Sleep(500 * time.Millisecond)

	viewer.join(voiceChannel)
	viewer.streamSilence()
	time.Sleep(500 * time.Millisecond)

	presenter.publishScreen(false)

	remote := viewer.awaitTrack(voice.SourceScreen, presenterID, 30*time.Second)
	readScreen(t, remote)
}
