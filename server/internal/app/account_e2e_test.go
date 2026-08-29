//go:build e2e

package app_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const deletedUsername = "Deleted User"

func (h *harness) deleteAccount(password string, wantStatus int) {
	h.t.Helper()
	h.mustDo(http.MethodDelete, "/api/v1/users/@me", wantStatus,
		map[string]string{"password": password}, nil)
}

func TestDeletingAnAccountKeepsWhatItSaid(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Leavers")
	text, _ := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	friend := owner.newUser()
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	var spoken events.Message
	friend.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "something worth keeping"}, &spoken)

	friend.deleteAccount("supersecret1", http.StatusNoContent)

	history := owner.history(text, "")
	if len(history) != 1 {
		t.Fatalf("history has %d messages, want the one the deleted account wrote", len(history))
	}
	if history[0].Content != "something worth keeping" {
		t.Errorf("content = %q, want it kept verbatim", history[0].Content)
	}
	if history[0].Author.ID != uuid.Nil {
		t.Errorf("author = %s, want the deleted-user tombstone", history[0].Author.ID)
	}
	if history[0].Author.Username != deletedUsername {
		t.Errorf("author name = %q, want %q", history[0].Author.Username, deletedUsername)
	}
}

func TestDeletingAnAccountEndsEveryWayBackIn(t *testing.T) {
	h := newHarness(t)
	_, refresh := h.registerUser()

	sock := h.dial()
	sock.identify(h.token)

	h.deleteAccount("supersecret1", http.StatusNoContent)

	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusUnauthorized, nil, nil)
	h.mustDo(http.MethodPost, "/api/v1/auth/refresh", http.StatusUnauthorized,
		map[string]string{"refresh_token": refresh}, nil)

	gone := &harness{t: t, server: h.server}
	gone.mustDo(http.MethodPost, "/api/v1/auth/login", http.StatusUnauthorized,
		map[string]string{"email": h.email, "password": "supersecret1"}, nil)

	started := time.Now()
	frame := sock.readAny()
	if frame != nil {
		t.Errorf("the gateway session survived the deletion and read %v", frame.Op)
	}
	if waited := time.Since(started); waited > 5*time.Second {
		t.Errorf("the socket stayed open for %s after the account was deleted; the session "+
			"was authenticated once at identify and is never checked again", waited)
	}
}

func TestDeletingAnOwnerHandsTheGuildOn(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Inherited")
	text, _ := owner.textAndVoice(guild.ID)

	invite := owner.createInvite(guild.ID, map[string]any{})
	heir := owner.newUser()
	heir.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	owner.deleteAccount("supersecret1", http.StatusNoContent)

	var guilds []events.Guild
	heir.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &guilds)
	if len(guilds) != 1 || guilds[0].ID != guild.ID {
		t.Fatalf("the remaining member lost the guild: %v", guilds)
	}
	if guilds[0].OwnerID == uuid.Nil {
		t.Fatal("ownership went to the tombstone rather than to a member")
	}

	heir.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "mine now"}, nil)
	heir.mustDo(http.MethodPost, "/api/v1/guilds/"+guild.ID.String()+"/invites",
		http.StatusCreated, map[string]any{}, nil)
}

func TestDeletingTheOnlyMemberTakesTheGuildWithIt(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Nobody Else")
	text, _ := h.textAndVoice(guild.ID)

	h.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "alone in here"}, nil)

	h.deleteAccount("supersecret1", http.StatusNoContent)

	stranger := newHarness(t)
	stranger.registerUser()
	stranger.mustDo(http.MethodGet, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusNotFound, nil, nil)
}

func TestDeletingAnAccountNeedsThePassword(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	h.deleteAccount("not-the-password", http.StatusUnauthorized)
	h.deleteAccount("", http.StatusUnauthorized)

	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusOK, nil, nil)
}

func TestNobodyCanRegisterAsTheDeletedUser(t *testing.T) {
	h := newHarness(t)

	h.mustDo(http.MethodPost, "/api/v1/auth/register", http.StatusConflict, map[string]string{
		"username": deletedUsername,
		"email":    "impostor" + randomSuffix() + "@example.test",
		"password": "supersecret1",
	}, nil)

	h.mustDo(http.MethodPost, "/api/v1/auth/login", http.StatusUnauthorized, map[string]string{
		"email":    "deleted-user@invalid.localhost",
		"password": "",
	}, nil)
}
