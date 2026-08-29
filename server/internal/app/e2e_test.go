//go:build e2e

package app_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func TestHealthz(t *testing.T) {
	h := newHarness(t)

	var out struct {
		Status string `json:"status"`
	}
	h.mustDo(http.MethodGet, "/healthz", http.StatusOK, nil, &out)
	if out.Status != "ok" {
		t.Errorf("status = %q, want ok", out.Status)
	}
}

func TestAuthFlow(t *testing.T) {
	h := newHarness(t)
	userID, refresh := h.registerUser()

	var me events.User
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusOK, nil, &me)
	if me.ID != userID {
		t.Errorf("@me returned %s, want %s", me.ID, userID)
	}

	saved := h.token
	h.token = ""
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusUnauthorized, nil, nil)
	h.token = "not-a-jwt"
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusUnauthorized, nil, nil)
	h.token = saved

	var rotated struct {
		AccessToken string `json:"access_token"`
	}
	h.mustDo(http.MethodPost, "/api/v1/auth/refresh", http.StatusOK,
		map[string]string{"refresh_token": refresh}, &rotated)
	if rotated.AccessToken == "" {
		t.Error("refresh returned no access token")
	}

	h.mustDo(http.MethodPost, "/api/v1/auth/refresh", http.StatusUnauthorized,
		map[string]string{"refresh_token": refresh}, nil)
}

func TestGuildCreationIsComplete(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	guild := h.createGuild("E2E Guild")
	channels := h.listChannels(guild.ID)

	if len(channels) != 2 {
		t.Fatalf("new guild has %d channels, want 2", len(channels))
	}
	kinds := map[string]bool{}
	for _, c := range channels {
		kinds[c.Kind] = true
	}
	if !kinds["text"] || !kinds["voice"] {
		t.Errorf("channel kinds = %v, want one text and one voice", kinds)
	}
}

func TestMessageLifecycle(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	guild := h.createGuild("E2E Messages")
	text, voice := h.textAndVoice(guild.ID)

	var created events.Message
	h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "hello world"}, &created)
	if created.Content != "hello world" {
		t.Fatalf("content = %q", created.Content)
	}

	for _, body := range []string{"one", "two", "three"} {
		h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
			http.StatusCreated, map[string]string{"content": body}, nil)
	}

	page := h.history(text, "?limit=2")
	if len(page) != 2 || page[0].Content != "three" || page[1].Content != "two" {
		t.Fatalf("first page = %v, want [three two] newest first", contents(page))
	}

	next := h.history(text, "?limit=2&before="+page[1].ID.String())
	if len(next) != 2 || next[0].Content != "one" || next[1].Content != "hello world" {
		t.Fatalf("second page = %v, want [one, hello world]", contents(next))
	}

	var edited events.Message
	h.mustDo(http.MethodPatch, "/api/v1/messages/"+created.ID.String(),
		http.StatusOK, map[string]string{"content": "edited"}, &edited)
	if edited.Content != "edited" || edited.EditedAt == nil {
		t.Errorf("edit did not apply: content=%q edited_at=%v", edited.Content, edited.EditedAt)
	}

	h.mustDo(http.MethodDelete, "/api/v1/messages/"+created.ID.String(), http.StatusNoContent, nil, nil)
	for _, m := range h.history(text, "") {
		if m.ID == created.ID {
			t.Error("deleted message still appears in history")
		}
	}

	h.mustDo(http.MethodPost, "/api/v1/channels/"+voice.String()+"/messages",
		http.StatusBadRequest, map[string]string{"content": "nope"}, nil)
	h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusBadRequest, map[string]string{"content": "   "}, nil)
	h.mustDo(http.MethodGet, "/api/v1/channels/not-a-uuid/messages", http.StatusBadRequest, nil, nil)
	h.mustDo(http.MethodGet, "/api/v1/channels/"+uuid.NewString()+"/messages", http.StatusNotFound, nil, nil)
}

func TestGatewayDeliversMessagesLive(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	guild := h.createGuild("E2E Gateway")
	text, _ := h.textAndVoice(guild.ID)

	sock := h.dial()
	ready := sock.identify(h.token)
	if len(ready.Guilds) != 1 || len(ready.Channels) != 2 {
		t.Fatalf("READY carried %d guilds and %d channels, want 1 and 2",
			len(ready.Guilds), len(ready.Channels))
	}

	sock.write(events.Frame{Op: events.OpHeartbeat})
	sock.readUntil("HEARTBEAT_ACK", func(f events.Frame) bool { return f.Op == events.OpHeartbeatAck })

	h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "over the wire"}, nil)

	var live events.Message
	decode(t, sock.readEvent(events.EventMessageCreate).D, &live)
	if live.Content != "over the wire" {
		t.Errorf("fanout delivered %q", live.Content)
	}
}

func TestGatewayResumeReplaysMissedFrames(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	guild := h.createGuild("E2E Resume")
	text, _ := h.textAndVoice(guild.ID)

	sock := h.dial()
	ready := sock.identify(h.token)

	h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "before the drop"}, nil)
	missed := sock.readEvent(events.EventMessageCreate)

	sock.conn.CloseNow()

	resumed := h.dial()
	if op := resumed.read().Op; op != events.OpHello {
		t.Fatalf("op = %d, want HELLO", op)
	}
	resumed.write(events.Frame{Op: events.OpResume, D: mustJSON(t, events.Resume{
		Token:     h.token,
		SessionID: ready.SessionID,
		Seq:       missed.S - 1,
	})})

	var replayed events.Message
	decode(t, resumed.readEvent(events.EventMessageCreate).D, &replayed)
	if replayed.Content != "before the drop" {
		t.Errorf("replayed %q, want the frame missed while disconnected", replayed.Content)
	}

	h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "after the resume"}, nil)

	live := resumed.readUntil("the post-resume message", func(f events.Frame) bool {
		if f.T != events.EventMessageCreate {
			return false
		}
		var m events.Message
		return json.Unmarshal(f.D, &m) == nil && m.Content == "after the resume"
	})
	if live.S <= missed.S {
		t.Errorf("sequence went backwards: %d after %d", live.S, missed.S)
	}
}

func TestGatewayRejectsUnusableSessions(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	bad := h.dial()
	bad.read()
	bad.write(events.Frame{Op: events.OpIdentify, D: mustJSON(t, events.Identify{Token: "garbage"})})
	if _, _, err := bad.conn.Read(t.Context()); err == nil {
		t.Error("gateway accepted a garbage token")
	}

	unknown := h.dial()
	unknown.read()
	unknown.write(events.Frame{Op: events.OpResume, D: mustJSON(t, events.Resume{
		Token:     h.token,
		SessionID: uuid.NewString(),
	})})
	if op := unknown.read().Op; op != events.OpInvalidSession {
		t.Errorf("op = %d, want INVALID_SESSION (%d)", op, events.OpInvalidSession)
	}
}

func TestPermissionsHideOtherPeoplesGuilds(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Private")
	text, _ := h.textAndVoice(guild.ID)

	outsider := newHarness(t)
	outsider.registerUser()

	outsider.mustDo(http.MethodGet, "/api/v1/guilds/"+guild.ID.String()+"/channels",
		http.StatusNotFound, nil, nil)
	outsider.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusNotFound, map[string]string{"content": "intruding"}, nil)

	var mine []events.Guild
	outsider.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &mine)
	for _, g := range mine {
		if g.ID == guild.ID {
			t.Error("a non-member sees the guild in their list")
		}
	}
}

func (h *harness) createGuild(name string) events.Guild {
	h.t.Helper()
	var guild events.Guild
	h.mustDo(http.MethodPost, "/api/v1/guilds", http.StatusCreated,
		map[string]string{"name": name}, &guild)
	return guild
}

func (h *harness) listChannels(guildID uuid.UUID) []events.Channel {
	h.t.Helper()
	var channels []events.Channel
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/channels",
		http.StatusOK, nil, &channels)
	return channels
}

func (h *harness) textAndVoice(guildID uuid.UUID) (text, voice uuid.UUID) {
	h.t.Helper()
	for _, c := range h.listChannels(guildID) {
		switch c.Kind {
		case "text":
			text = c.ID
		case "voice":
			voice = c.ID
		}
	}
	if text == uuid.Nil || voice == uuid.Nil {
		h.t.Fatal("guild is missing its default channels")
	}
	return text, voice
}

func (h *harness) history(channelID uuid.UUID, query string) []events.Message {
	h.t.Helper()
	var msgs []events.Message
	h.mustDo(http.MethodGet, "/api/v1/channels/"+channelID.String()+"/messages"+query,
		http.StatusOK, nil, &msgs)
	return msgs
}

func contents(msgs []events.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Content
	}
	return out
}
