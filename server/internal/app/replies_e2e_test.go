//go:build e2e

package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) reply(channelID, parentID uuid.UUID, content string, wantStatus int) events.Message {
	h.t.Helper()
	var msg events.Message
	h.mustDo(http.MethodPost, "/api/v1/channels/"+channelID.String()+"/messages",
		wantStatus, map[string]string{"content": content, "reply_to": parentID.String()}, &msg)
	return msg
}

func TestAReplyCarriesEnoughOfWhatItAnswers(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Replies")
	text, _ := h.textAndVoice(guild.ID)

	parent := h.send(text, "the original point")
	answer := h.reply(text, parent.ID, "answering that", http.StatusCreated)

	if answer.ReplyTo == nil {
		t.Fatal("the reply came back without the message it answers")
	}
	if answer.ReplyTo.MessageID != parent.ID {
		t.Errorf("reply_to = %s, want %s", answer.ReplyTo.MessageID, parent.ID)
	}
	if answer.ReplyTo.Content != "the original point" {
		t.Errorf("preview content = %q", answer.ReplyTo.Content)
	}
	if answer.ReplyTo.Author == nil || answer.ReplyTo.Author.ID != parent.Author.ID {
		t.Errorf("preview author = %+v, want the parent's author", answer.ReplyTo.Author)
	}
	if answer.ReplyTo.Truncated || answer.ReplyTo.Deleted || answer.ReplyTo.HasAttachments {
		t.Errorf("preview flags wrong on a short live parent: %+v", answer.ReplyTo)
	}

	seen := firstMessage(t, h.history(text, ""), answer.ID)
	if seen.ReplyTo == nil || seen.ReplyTo.Content != "the original point" {
		t.Fatalf("history lost the preview: %+v", seen.ReplyTo)
	}

	plain := firstMessage(t, h.history(text, ""), parent.ID)
	if plain.ReplyTo != nil {
		t.Errorf("a message answering nothing carries %+v", plain.ReplyTo)
	}
}

func TestAReplyReachesAWatcherWithItsPreview(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Live Replies")
	text, _ := h.textAndVoice(guild.ID)
	parent := h.send(text, "say that again")

	watcher := h.dial()
	watcher.identify(h.token)

	h.reply(text, parent.ID, "gladly", http.StatusCreated)

	var live events.Message
	decode(t, watcher.readEvent(events.EventMessageCreate).D, &live)
	if live.ReplyTo == nil || live.ReplyTo.Content != "say that again" {
		t.Fatalf("MESSAGE_CREATE carries %+v; a live reply must render without a refetch", live.ReplyTo)
	}
}

func TestADeletedParentKeepsItsWordsToItself(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Deleted Parents")
	text, _ := h.textAndVoice(guild.ID)

	parent := h.send(text, "something regretted")
	answer := h.reply(text, parent.ID, "quoting it", http.StatusCreated)
	h.mustDo(http.MethodDelete, "/api/v1/messages/"+parent.ID.String(), http.StatusNoContent, nil, nil)

	seen := firstMessage(t, h.history(text, ""), answer.ID)
	if seen.ReplyTo == nil {
		t.Fatal("deleting the parent took the reply's pointer with it")
	}
	if !seen.ReplyTo.Deleted {
		t.Error("the preview does not say the parent is gone")
	}
	if seen.ReplyTo.Content != "" {
		t.Errorf("preview content = %q; a deleted message must not come back through a reply",
			seen.ReplyTo.Content)
	}
	if seen.ReplyTo.Author != nil {
		t.Errorf("preview author = %+v; a deleted message must not name who wrote it",
			seen.ReplyTo.Author)
	}
}

func TestAReplyCannotReachIntoAnotherChannel(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Elsewhere")
	text, _ := h.textAndVoice(guild.ID)
	other := h.newTextChannel(guild.ID, "other")

	parent := h.send(other, "over here")
	h.reply(text, parent.ID, "answering across channels", http.StatusBadRequest)
	h.reply(text, uuid.Must(uuid.NewV7()), "answering nothing at all", http.StatusBadRequest)
}

func TestAReplyToAReplyShowsOneLevel(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("One Level")
	text, _ := h.textAndVoice(guild.ID)

	first := h.send(text, "one")
	second := h.reply(text, first.ID, "two", http.StatusCreated)
	third := h.reply(text, second.ID, "three", http.StatusCreated)

	if third.ReplyTo == nil || third.ReplyTo.MessageID != second.ID {
		t.Fatalf("the third message answers %+v, want the second", third.ReplyTo)
	}
	if third.ReplyTo.Content != "two" {
		t.Errorf("preview content = %q, want the immediate parent", third.ReplyTo.Content)
	}
}

func TestALongParentIsTruncatedRatherThanSentWhole(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Long Parents")
	text, _ := h.textAndVoice(guild.ID)

	long := strings.Repeat("x", 3000)
	parent := h.send(text, long)
	answer := h.reply(text, parent.ID, "tl;dr", http.StatusCreated)

	if answer.ReplyTo == nil {
		t.Fatal("no preview came back")
	}
	if len(answer.ReplyTo.Content) >= len(long) {
		t.Fatalf("preview is %d characters; fifty of these is a page nobody can load",
			len(answer.ReplyTo.Content))
	}
	if !answer.ReplyTo.Truncated {
		t.Error("the preview was cut but does not say so")
	}
}

func TestAReplyMentionsThatItsParentCarriedAFile(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Parent With A File")
	text, _ := h.textAndVoice(guild.ID)

	_, parent := h.postFiles(text, "see attached", map[string][]byte{"notes.txt": []byte("hi")})
	answer := h.reply(text, parent.ID, "got it", http.StatusCreated)

	if answer.ReplyTo == nil || !answer.ReplyTo.HasAttachments {
		t.Fatalf("preview = %+v; a quoted message with a file should say so", answer.ReplyTo)
	}
}

func TestEditingAReplyKeepsWhatItAnswers(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Edited Replies")
	text, _ := h.textAndVoice(guild.ID)

	parent := h.send(text, "the question")
	answer := h.reply(text, parent.ID, "first attempt", http.StatusCreated)

	watcher := h.dial()
	watcher.identify(h.token)

	var edited events.Message
	h.mustDo(http.MethodPatch, "/api/v1/messages/"+answer.ID.String(), http.StatusOK,
		map[string]string{"content": "second attempt"}, &edited)
	if edited.ReplyTo == nil || edited.ReplyTo.MessageID != parent.ID {
		t.Errorf("editing a reply dropped what it answers: %+v", edited.ReplyTo)
	}

	var broadcast events.Message
	decode(t, watcher.readEvent(events.EventMessageUpdate).D, &broadcast)
	if broadcast.ReplyTo == nil || broadcast.ReplyTo.Content != "the question" {
		t.Errorf("MESSAGE_UPDATE carries %+v; everybody else would lose the quote",
			broadcast.ReplyTo)
	}
}
