//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func channelIn(t *testing.T, ready events.Ready, channelID uuid.UUID) events.Channel {
	t.Helper()
	for _, ch := range ready.Channels {
		if ch.ID == channelID {
			return ch
		}
	}
	t.Fatalf("READY carried no channel %s", channelID)
	return events.Channel{}
}

func readStateFor(ready events.Ready, channelID uuid.UUID) (events.ReadState, bool) {
	for _, state := range ready.ReadStates {
		if state.ChannelID == channelID {
			return state, true
		}
	}
	return events.ReadState{}, false
}

func isOnline(ready events.Ready, userID uuid.UUID) bool {
	for _, id := range ready.Online {
		if id == userID {
			return true
		}
	}
	return false
}

func isMember(ready events.Ready, guildID, userID uuid.UUID) bool {
	for _, m := range ready.Members {
		if m.GuildID == guildID && m.User.ID == userID {
			return true
		}
	}
	return false
}

func post(h *harness, channelID uuid.UUID, content string) events.Message {
	h.t.Helper()
	var created events.Message
	h.mustDo(http.MethodPost, "/api/v1/channels/"+channelID.String()+"/messages",
		http.StatusCreated, map[string]string{"content": content}, &created)
	return created
}

func TestReadyNamesTheNewestMessageInEachChannel(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Unreads")
	text, voiceChannel := owner.textAndVoice(guild.ID)

	post(owner, text, "first")
	newest := post(owner, text, "second")

	sock := owner.dial()
	ready := sock.identify(owner.token)

	channel := channelIn(t, ready, text)
	if channel.LastMessageID == nil {
		t.Fatal("READY named no newest message for a channel that has one, so unread is not " +
			"computable at all without a request per channel")
	}
	if *channel.LastMessageID != newest.ID {
		t.Errorf("last_message_id = %s, want the newest message %s", *channel.LastMessageID, newest.ID)
	}
	if empty := channelIn(t, ready, voiceChannel); empty.LastMessageID != nil {
		t.Errorf("a channel with no messages named %s as its newest", *empty.LastMessageID)
	}
}

func TestReadyLeavesAnUnopenedChannelWithoutAReadState(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Never Opened")
	text, _ := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	post(owner, text, "anyone there?")

	ready := friend.dial().identify(friend.token)

	if _, ok := readStateFor(ready, text); ok {
		t.Error("a member who has never opened the channel arrived with a read state for it; " +
			"the absence of one is what marks everything unread on a first connect")
	}
	if channelIn(t, ready, text).LastMessageID == nil {
		t.Error("the channel carried no newest message, so nothing marks it unread either")
	}
}

func TestReadyCarriesTheReadStateOfAnOpenedChannel(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Opened")
	text, _ := owner.textAndVoice(guild.ID)

	seen := post(owner, text, "read me")
	owner.mustDo(http.MethodPut, "/api/v1/channels/"+text.String()+"/read",
		http.StatusNoContent, map[string]string{"message_id": seen.ID.String()}, nil)

	ready := owner.dial().identify(owner.token)

	state, ok := readStateFor(ready, text)
	if !ok {
		t.Fatal("READY carried no read state for a channel that was marked read, so a client " +
			"shows an unread badge on a channel the member has already seen")
	}
	if state.LastReadMessageID == nil || *state.LastReadMessageID != seen.ID {
		t.Errorf("last_read_message_id = %v, want %s", state.LastReadMessageID, seen.ID)
	}

	channel := channelIn(t, ready, text)
	if channel.LastMessageID == nil || *channel.LastMessageID != *state.LastReadMessageID {
		t.Error("the newest message and the last read one disagree on a channel nobody has " +
			"posted to since it was read")
	}
}

func awaitPresence(t *testing.T, s *socket, userID uuid.UUID) string {
	t.Helper()
	for range 10 {
		var update events.PresenceUpdate
		decode(t, s.readEvent(events.EventPresenceUpdate).D, &update)
		if update.UserID == userID {
			return update.Status
		}
	}
	t.Fatalf("no presence update about %s arrived", userID)
	return ""
}

func TestReadyNamesFellowMembersAndWhoIsOnline(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Company")

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	stranger := owner.newUser()
	strangerID := stranger.dial().identify(stranger.token).User.ID

	held := owner.dial()
	held.identify(owner.token)

	ready := friend.dial().identify(friend.token)

	if !isMember(ready, guild.ID, ownerID) {
		t.Fatal("READY named no fellow members, so a client cannot put a name against a user id " +
			"it later sees speaking, typing or coming online")
	}
	if isMember(ready, guild.ID, strangerID) {
		t.Error("READY named someone who is not in the guild")
	}
	if !isOnline(ready, ownerID) {
		t.Error("a member with a live session was missing from the online set, so a client that " +
			"joins a populated guild shows everyone as offline until they next change state")
	}
	if !isOnline(ready, ready.User.ID) {
		t.Error("the connecting member was not in their own online set")
	}
	if isOnline(ready, strangerID) {
		t.Error("READY reported the presence of someone outside the guild")
	}
}

func TestReadySnapshotFollowsTheSamePresenceRuleAsTheEvents(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Departures")

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	watcher := friend.dial()
	watcher.identify(friend.token)

	held := owner.dial()
	held.identify(owner.token)

	if status := awaitPresence(t, watcher, ownerID); status != "online" {
		t.Fatalf("the owner came online as %q", status)
	}

	held.conn.CloseNow()

	if !isOnline(friend.dial().identify(friend.token), ownerID) {
		t.Error("a dropped socket took the owner out of the presence snapshot straight away, " +
			"while PRESENCE_UPDATE waits out the resume window before saying they left. The two " +
			"would disagree for the whole of that window, and every reconnect would flicker")
	}
}
