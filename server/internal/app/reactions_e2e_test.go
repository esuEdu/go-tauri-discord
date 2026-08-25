//go:build e2e

package app_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/config"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func reactionPath(messageID uuid.UUID, emoji string) string {
	return "/api/v1/messages/" + messageID.String() + "/reactions/" + url.PathEscape(emoji)
}

func (h *harness) send(channelID uuid.UUID, content string) events.Message {
	h.t.Helper()
	var msg events.Message
	h.mustDo(http.MethodPost, "/api/v1/channels/"+channelID.String()+"/messages",
		http.StatusCreated, map[string]string{"content": content}, &msg)
	return msg
}

func (h *harness) react(messageID uuid.UUID, emoji string) {
	h.t.Helper()
	h.mustDo(http.MethodPut, reactionPath(messageID, emoji), http.StatusNoContent, nil, nil)
}

func (h *harness) unreact(messageID uuid.UUID, emoji string) {
	h.t.Helper()
	h.mustDo(http.MethodDelete, reactionPath(messageID, emoji), http.StatusNoContent, nil, nil)
}

func reactionOf(msg events.Message, emoji string) (events.Reaction, bool) {
	for _, r := range msg.Reactions {
		if r.Emoji == emoji {
			return r, true
		}
	}
	return events.Reaction{}, false
}

func firstMessage(t *testing.T, msgs []events.Message, id uuid.UUID) events.Message {
	t.Helper()
	for _, m := range msgs {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("message %s is missing from history", id)
	return events.Message{}
}

func TestHistoryCountsReactionsAndSaysWhichAreMine(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Reactions")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)

	posted := owner.send(text, "worth a thumb")
	owner.react(posted.ID, "👍")
	member.react(posted.ID, "👍")
	member.react(posted.ID, "🎉")

	seen := firstMessage(t, owner.history(text, ""), posted.ID)
	thumbs, ok := reactionOf(seen, "👍")
	if !ok {
		t.Fatalf("history carries no 👍; reactions = %+v", seen.Reactions)
	}
	if thumbs.Count != 2 {
		t.Errorf("👍 count = %d, want 2", thumbs.Count)
	}
	if !thumbs.Mine {
		t.Error("the owner reacted with 👍 but history says it is not theirs")
	}

	party, ok := reactionOf(seen, "🎉")
	if !ok {
		t.Fatal("history carries no 🎉")
	}
	if party.Mine {
		t.Error("🎉 is only the member's, but the owner is told it is theirs")
	}

	byMember := firstMessage(t, member.history(text, ""), posted.ID)
	if party, _ := reactionOf(byMember, "🎉"); !party.Mine {
		t.Error("the member reacted with 🎉 but their own history says otherwise")
	}
}

func TestTheSamePersonCannotReactTwiceWithTheSameEmoji(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Once Only")
	text, _ := h.textAndVoice(guild.ID)

	posted := h.send(text, "count me once")
	h.react(posted.ID, "🔥")
	h.react(posted.ID, "🔥")

	seen := firstMessage(t, h.history(text, ""), posted.ID)
	fire, ok := reactionOf(seen, "🔥")
	if !ok {
		t.Fatal("the reaction did not survive")
	}
	if fire.Count != 1 {
		t.Fatalf("🔥 count = %d after reacting twice, want 1", fire.Count)
	}

	h.unreact(posted.ID, "🔥")
	gone := firstMessage(t, h.history(text, ""), posted.ID)
	if _, still := reactionOf(gone, "🔥"); still {
		t.Error("the reaction outlived its removal")
	}
	if gone.Reactions == nil {
		t.Error("a message with no reactions must carry an empty list, not null")
	}
}

func TestAReactionReachesEverybodyWatchingTheChannel(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Live Reactions")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	watcher := owner.dial()
	watcher.identify(owner.token)

	posted := owner.send(text, "react to me")
	watcher.readEvent(events.EventMessageCreate)

	member.react(posted.ID, "👀")

	var added events.MessageReaction
	decode(t, watcher.readEvent(events.EventReactionAdd).D, &added)
	if added.MessageID != posted.ID || added.Emoji != "👀" || added.UserID != memberID {
		t.Fatalf("MESSAGE_REACTION_ADD = %+v, want the member's 👀 on %s", added, posted.ID)
	}
	if added.ChannelID != text {
		t.Errorf("the event carries channel %s, want %s; without it the gateway cannot hide it",
			added.ChannelID, text)
	}

	member.unreact(posted.ID, "👀")

	var removed events.MessageReaction
	decode(t, watcher.readEvent(events.EventReactionRemove).D, &removed)
	if removed.MessageID != posted.ID || removed.Emoji != "👀" || removed.UserID != memberID {
		t.Fatalf("MESSAGE_REACTION_REMOVE = %+v, want the member taking back 👀", removed)
	}
}

func TestAReactionInAHiddenChannelStaysHidden(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Hidden Reactions")
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	secret := owner.newTextChannel(guild.ID, "secret")
	owner.denyView(secret, memberID, domain.OverwriteMember)

	watcher := member.dial()
	watcher.identify(member.token)

	posted := owner.send(secret, "not for you")
	owner.react(posted.ID, "🤫")

	if !watcher.quietFor(time.Second, func(f events.Frame) bool {
		return f.T == events.EventReactionAdd || f.T == events.EventMessageCreate
	}) {
		t.Fatal("a reaction in a channel the member cannot see reached them anyway")
	}

	member.mustDo(http.MethodPut, reactionPath(posted.ID, "🤫"), http.StatusForbidden, nil, nil)
}

func TestReactingNeedsThePermissionButTakingItBackDoesNot(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Reaction Permission")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)
	everyone := owner.everyone(guild.ID)

	posted := owner.send(text, "while you still can")
	member.react(posted.ID, "❤️")

	owner.mustDo(http.MethodPatch, "/api/v1/roles/"+everyone.ID.String(), http.StatusOK,
		map[string]any{
			"permissions": perm(domain.DefaultEveryonePermissions &^ domain.PermAddReactions),
		}, nil)

	member.mustDo(http.MethodPut, reactionPath(posted.ID, "🎈"), http.StatusForbidden, nil, nil)
	member.unreact(posted.ID, "❤️")

	left := firstMessage(t, owner.history(text, ""), posted.ID)
	if len(left.Reactions) != 0 {
		t.Fatalf("reactions = %+v; losing the permission must not trap the old one", left.Reactions)
	}
}

func TestAReactionIsAnEmojiNotAnEssay(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Emoji Only")
	text, _ := h.textAndVoice(guild.ID)
	posted := h.send(text, "try it")

	for _, rejected := range []string{"lol", "👍 👎", "👍👎🎉❤️🔥👀✅🎈💯"} {
		if got := h.do(http.MethodPut, reactionPath(posted.ID, rejected), nil, nil); got != http.StatusBadRequest {
			t.Errorf("reacting with %q returned %d, want 400", rejected, got)
		}
	}
}

func TestAMessageStopsAcceptingNewKindsOfReaction(t *testing.T) {
	h := newHarness(t, func(c *config.Config) { c.RateLimitDisabled = true })
	h.registerUser()
	guild := h.createGuild("Reaction Cap")
	text, _ := h.textAndVoice(guild.ID)
	posted := h.send(text, "pile on")

	kinds := []string{
		"👍", "👎", "😄", "🎉", "❤️", "👀", "🔥", "✅", "🎈", "💯",
		"🙏", "🚀", "🐛", "🍕", "☕", "🌈", "⚡", "🧠", "🎯", "🥳",
	}
	for _, emoji := range kinds {
		h.react(posted.ID, emoji)
	}
	if got := h.do(http.MethodPut, reactionPath(posted.ID, "🛑"), nil, nil); got != http.StatusBadRequest {
		t.Errorf("the twenty-first kind of reaction returned %d, want 400", got)
	}

	h.inviteMember(guild.ID).react(posted.ID, "👍")

	seen := firstMessage(t, h.history(text, ""), posted.ID)
	if len(seen.Reactions) != len(kinds) {
		t.Fatalf("message carries %d kinds, want %d", len(seen.Reactions), len(kinds))
	}
	if thumbs, _ := reactionOf(seen, "👍"); thumbs.Count != 2 {
		t.Errorf("👍 count = %d; the cap is on kinds, not on people", thumbs.Count)
	}
}

func TestWhoReactedIsAskedForSeparately(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Who Reacted")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)
	memberID, _ := memberIdentity(t, member)

	posted := owner.send(text, "hover over it")
	member.react(posted.ID, "🙏")

	var people []events.User
	owner.mustDo(http.MethodGet, reactionPath(posted.ID, "🙏"), http.StatusOK, nil, &people)
	if len(people) != 1 || people[0].ID != memberID {
		t.Fatalf("who reacted = %+v, want just the member", people)
	}
}

func TestEditingAMessageKeepsItsFilesAndReactions(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Edit Keeps")
	text, _ := owner.textAndVoice(guild.ID)

	_, posted := owner.postFiles(text, "before", map[string][]byte{"notes.txt": []byte("kept")})
	if len(posted.Attachments) != 1 {
		t.Fatal("the message did not carry its file")
	}
	owner.react(posted.ID, "✅")

	watcher := owner.dial()
	watcher.identify(owner.token)

	var edited events.Message
	owner.mustDo(http.MethodPatch, "/api/v1/messages/"+posted.ID.String(), http.StatusOK,
		map[string]string{"content": "after"}, &edited)
	if len(edited.Attachments) != 1 {
		t.Errorf("editing dropped the file from the reply; attachments = %+v", edited.Attachments)
	}

	var broadcast events.Message
	decode(t, watcher.readEvent(events.EventMessageUpdate).D, &broadcast)
	if len(broadcast.Attachments) != 1 {
		t.Errorf("MESSAGE_UPDATE carries %d attachments; everybody else would lose the file",
			len(broadcast.Attachments))
	}

	seen := firstMessage(t, owner.history(text, ""), posted.ID)
	if seen.Content != "after" {
		t.Errorf("content = %q, want the edit", seen.Content)
	}
	if _, ok := reactionOf(seen, "✅"); !ok {
		t.Error("editing a message threw away its reactions")
	}
}
