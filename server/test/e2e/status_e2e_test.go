//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) setStatus(body map[string]any, want int) events.Ready {
	h.t.Helper()
	var out struct {
		Self events.Self `json:"self"`
	}
	h.mustDo(http.MethodPatch, "/api/v1/users/@me", want, body, &out)
	return events.Ready{Self: out.Self}
}

func (h *harness) rosterStatus(guildID uuid.UUID, who events.UserID) (string, *string) {
	h.t.Helper()
	var rows []struct {
		UserID       events.UserID `json:"user_id"`
		Status       string        `json:"status"`
		CustomStatus *string       `json:"custom_status"`
	}
	h.mustDo(http.MethodGet, "/api/v1/guilds/"+guildID.String()+"/members",
		http.StatusOK, nil, &rows)
	for _, r := range rows {
		if r.UserID == who {
			return r.Status, r.CustomStatus
		}
	}
	h.t.Fatalf("no roster row for %s", who)
	return "", nil
}

func TestAChosenStatusSurvivesAReconnect(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	if got := h.setStatus(map[string]any{"status": "busy"}, http.StatusOK).Self.Status; got != "busy" {
		t.Fatalf("status = %q, want busy", got)
	}

	ready := h.dial().identify(h.token)
	if ready.Self.Status != "busy" {
		t.Errorf("a fresh session forgot the chosen status: %q", ready.Self.Status)
	}
	if got := statusIn(ready, ready.User.ID); got != "busy" {
		t.Errorf("READY reported you as %q, want busy", got)
	}
}

func TestOfflineCannotBeChosenOverTheWire(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	h.setStatus(map[string]any{"status": "offline"}, http.StatusBadRequest)
	h.setStatus(map[string]any{"status": "lurking"}, http.StatusBadRequest)
}

func TestInvisibleTellsTheSameLieEverywhere(t *testing.T) {
	owner := newHarness(t)
	ownerID, _ := owner.registerUser()
	guild := owner.createGuild("Hidden")

	friend := owner.inviteMember(guild.ID)
	friendID := friend.whoAmI()

	watcher := owner.dial()
	watcher.identify(owner.token)

	ghost := friend.dial()
	ghost.identify(friend.token)
	if got := awaitPresence(t, watcher, friendID); got != "online" {
		t.Fatalf("a connected member read as %q before going invisible", got)
	}

	friend.setStatus(map[string]any{"status": "invisible", "custom_status": "here really"}, http.StatusOK)

	if got := awaitPresence(t, watcher, friendID); got != "offline" {
		t.Errorf("PRESENCE_UPDATE gave an invisible member away as %q", got)
	}

	ready := owner.dial().identify(owner.token)
	if got := statusIn(ready, friendID); got != "offline" {
		t.Errorf("the READY snapshot gave an invisible member away as %q", got)
	}
	for _, p := range ready.Presence {
		if p.UserID == friendID && p.CustomStatus != nil {
			t.Errorf("an invisible member's status text reached somebody else: %q", *p.CustomStatus)
		}
	}

	status, custom := owner.rosterStatus(guild.ID, friendID)
	if status != "offline" {
		t.Errorf("the member roster gave an invisible member away as %q", status)
	}
	if custom != nil {
		t.Errorf("the roster carried an invisible member's status text: %q", *custom)
	}

	if self := friend.setStatus(nil, http.StatusOK); self.Self.Status != "invisible" {
		t.Errorf("the invisible person cannot see their own choice: %q", self.Self.Status)
	}

	_ = ownerID
	_ = ghost
}

func TestCustomStatusReachesOthersAndCanBeCleared(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Sayings")
	friend := owner.inviteMember(guild.ID)
	friendID := friend.whoAmI()

	watcher := owner.dial()
	watcher.identify(owner.token)
	friend.dial().identify(friend.token)
	awaitPresence(t, watcher, friendID)

	friend.setStatus(map[string]any{"custom_status": "  writing tests  "}, http.StatusOK)
	_, custom := owner.rosterStatus(guild.ID, friendID)
	if custom == nil || *custom != "writing tests" {
		t.Fatalf("custom status = %v, want it trimmed and carried", custom)
	}

	friend.setStatus(map[string]any{"custom_status": ""}, http.StatusOK)
	if _, cleared := owner.rosterStatus(guild.ID, friendID); cleared != nil {
		t.Errorf("an emptied status text was kept: %q", *cleared)
	}
}

func TestAStatusTooLongIsRefused(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	long := make([]byte, 129)
	for i := range long {
		long[i] = 'a'
	}
	h.setStatus(map[string]any{"custom_status": string(long)}, http.StatusBadRequest)
}

func TestGoingIdleReadsAsAwayAndOnlyWhileOnline(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Quiet hours")
	friend := owner.inviteMember(guild.ID)
	friendID := friend.whoAmI()

	watcher := owner.dial()
	watcher.identify(owner.token)

	sleeper := friend.dial()
	sleeper.identify(friend.token)
	awaitPresence(t, watcher, friendID)

	sleeper.write(events.Frame{Op: events.OpPresence, D: mustJSON(t, events.PresenceRequest{Idle: true})})
	if got := awaitPresence(t, watcher, friendID); got != "away" {
		t.Errorf("an idle member read as %q, want away; there is no fifth value on the wire", got)
	}

	friend.setStatus(map[string]any{"status": "busy"}, http.StatusOK)
	if got := awaitPresence(t, watcher, friendID); got != "busy" {
		t.Errorf("idleness overrode a chosen status: %q", got)
	}
}
