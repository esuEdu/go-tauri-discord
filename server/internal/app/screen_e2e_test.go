//go:build e2e

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		select {
		case remote := <-viewer.remote:
			if remote.Kind() != webrtc.RTPCodecTypeVideo {
				continue
			}
			buf := make([]byte, 1500)
			if err := remote.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := remote.Read(buf); err != nil {
				t.Fatalf("no RTP arrived on the forwarded screen track: %v", err)
			}
			if mid := viewer.midOf(remote); mid == viewer.reservedMid() {
				t.Fatalf("the presenter's screen arrived on mid %q, which is the viewer's own "+
					"upload slot; a viewer that sets a direction on that slot silences the share", mid)
			}
			if id := remote.StreamID(); id == "" || id == "-" {
				t.Fatalf("the forwarded screen carried stream id %q, so a browser reports no "+
					"stream on the track event and the viewer drops it before rendering", id)
			}
			return
		case <-ctx.Done():
			t.Fatal("the viewer never received the presenter's screen through the SFU")
		}
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

	deadline := time.After(2 * time.Second)
	seen := 0
	for seen < 2 {
		select {
		case <-presenter.keyframes:
			seen++
		case <-deadline:
			t.Fatalf("the presenter got %d keyframe requests in the two seconds after a viewer "+
				"subscribed; the idle ticker alone can produce one, so the subscribe path is not "+
				"asking and the viewer waits for a natural keyframe while showing black", seen)
		}
	}
}
