//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/voice"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) internalID(public events.UserID) uuid.UUID {
	h.t.Helper()

	var id uuid.UUID
	err := h.pool.Raw().QueryRow(context.Background(),
		"SELECT id FROM users WHERE public_id = $1", string(public)).Scan(&id)
	if err != nil {
		h.t.Fatalf("no user row for public id %q: %v", public, err)
	}
	return id
}

func TestAPublicIDIsNotTheRowID(t *testing.T) {
	h := newHarness(t)
	public, _ := h.registerUser()

	if !voice.ValidPublicID(string(public)) {
		t.Fatalf("public id %q is not the published shape; the client splits track names on it",
			public)
	}
	if _, err := uuid.Parse(string(public)); err == nil {
		t.Errorf("public id %q parses as a uuid, so a leak of the row id would be invisible", public)
	}
	if got := h.internalID(public); got.String() == string(public) {
		t.Error("the public id and the row id are the same value")
	}
}

func TestReadyCarriesNoRowIDs(t *testing.T) {
	owner := newHarness(t, withRelay(t, "shared-with-coturn"))
	ownerPublic, _ := owner.registerUser()
	guild := owner.createGuild("Quiet")
	text, _ := owner.textAndVoice(guild.ID)

	member := owner.inviteMember(guild.ID)
	memberPublic := member.whoAmI()
	member.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "hello"}, nil)

	hidden := []uuid.UUID{
		owner.internalID(ownerPublic),
		owner.internalID(memberPublic),
	}

	sock := owner.dial()
	frame := sock.identifyRaw(owner.token)
	body := string(frame.D)

	for _, id := range hidden {
		if strings.Contains(body, id.String()) {
			t.Errorf("READY carries the row id %s; the identifier in the token must never "+
				"reach another client", id)
		}
	}

	var ready events.Ready
	if err := json.Unmarshal(frame.D, &ready); err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if len(ready.ICEServers) < 2 || ready.ICEServers[1].Username == "" {
		t.Fatal("no relay credentials were minted, so this test cannot see them carrying an id")
	}
	if ready.User.ID != ownerPublic {
		t.Errorf("READY named you %q, want %q", ready.User.ID, ownerPublic)
	}
	for _, m := range ready.Members {
		if _, err := uuid.Parse(string(m.User.ID)); err == nil {
			t.Errorf("member %s is identified by a uuid", m.User.ID)
		}
	}
}

func TestModerationAddressesPeopleByPublicID(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Rules")

	member := owner.inviteMember(guild.ID)
	memberPublic := member.whoAmI()
	rowID := owner.internalID(memberPublic)

	owner.mustDo(http.MethodDelete, memberPath(guild.ID, events.UserID(rowID.String())),
		http.StatusNotFound, nil, nil)
	if !member.inGuild(guild.ID) {
		t.Fatal("a kick addressed by row id succeeded; the row id must not be an address")
	}

	owner.mustDo(http.MethodDelete, memberPath(guild.ID, memberPublic),
		http.StatusNoContent, nil, nil)
	if member.inGuild(guild.ID) {
		t.Error("a kick addressed by public id did not remove the member")
	}
}
