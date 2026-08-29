//go:build e2e

package app_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) whoAmI() uuid.UUID {
	h.t.Helper()
	var me events.User
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusOK, nil, &me)
	return me.ID
}

func TestChangingYourPictureTellsTheServersYouAreIn(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Faces")
	member := owner.inviteMember(guild.ID)
	memberID := member.whoAmI()

	sock := owner.dial()
	sock.identify(owner.token)

	if status, _ := member.putImage("/api/v1/users/@me/avatar",
		aPNG(t, 96, 96), "image/png"); status != http.StatusOK {
		t.Fatalf("PUT avatar = %d, want 200", status)
	}

	var heard events.User
	decode(t, sock.readEvent(events.EventUserUpdate).D, &heard)
	if heard.ID != memberID {
		t.Fatalf("event carried user %s, want the member %s", heard.ID, memberID)
	}
	if heard.AvatarKey == nil || *heard.AvatarKey == "" {
		t.Error("the event carried no avatar key, so nobody can fetch the new picture")
	}
	if heard.Username == "" {
		t.Error("the event carried no username, so a roster cannot use it")
	}
}

func TestClearingYourPictureIsAnnouncedToo(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Plain Faces")
	member := owner.inviteMember(guild.ID)
	memberID := member.whoAmI()

	member.putImage("/api/v1/users/@me/avatar", aPNG(t, 96, 96), "image/png")

	sock := owner.dial()
	sock.identify(owner.token)

	member.mustDo(http.MethodDelete, "/api/v1/users/@me/avatar",
		http.StatusNoContent, nil, nil)

	var heard events.User
	decode(t, sock.readEvent(events.EventUserUpdate).D, &heard)
	if heard.ID != memberID {
		t.Fatalf("event carried user %s, want the member %s", heard.ID, memberID)
	}
	if heard.AvatarKey != nil {
		t.Errorf("avatar_key = %q, clearing should announce an empty picture", *heard.AvatarKey)
	}
}

func TestAStrangerIsNotToldAboutYourPicture(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	me.createGuild("Mine Alone")

	stranger := newHarness(t)
	stranger.registerUser()
	stranger.createGuild("Theirs Alone")

	sock := stranger.dial()
	sock.identify(stranger.token)

	me.putImage("/api/v1/users/@me/avatar", aPNG(t, 96, 96), "image/png")

	quiet := sock.quietFor(700*time.Millisecond, func(f events.Frame) bool {
		return f.T == events.EventUserUpdate
	})
	if !quiet {
		t.Error("a stranger was told about somebody else's new picture")
	}
}
