//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) guildOrder() []string {
	h.t.Helper()
	var mine []events.Guild
	h.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &mine)

	names := make([]string, 0, len(mine))
	for _, g := range mine {
		names = append(names, g.Name)
	}
	return names
}

func (h *harness) reorder(order []uuid.UUID, want int) []string {
	h.t.Helper()
	var back []events.Guild
	into := any(&back)
	if want != http.StatusOK {
		into = nil
	}
	h.mustDo(http.MethodPut, "/api/v1/guilds/order", want,
		map[string]any{"guild_ids": order}, into)

	names := make([]string, 0, len(back))
	for _, g := range back {
		names = append(names, g.Name)
	}
	return names
}

func TestTheServerRailKeepsTheOrderYouDragItInto(t *testing.T) {
	me := newHarness(t)
	me.registerUser()

	first := me.createGuild("Alpha")
	second := me.createGuild("Beta")
	third := me.createGuild("Gamma")

	if got := me.guildOrder(); !same(got, []string{"Alpha", "Beta", "Gamma"}) {
		t.Fatalf("a fresh list = %v, want them in the order they were made", got)
	}

	echoed := me.reorder([]uuid.UUID{third.ID, first.ID, second.ID}, http.StatusOK)
	if !same(echoed, []string{"Gamma", "Alpha", "Beta"}) {
		t.Errorf("the reorder reply = %v, want the new order in the same shape GET returns", echoed)
	}

	if got := me.guildOrder(); !same(got, []string{"Gamma", "Alpha", "Beta"}) {
		t.Errorf("after reordering = %v, want [Gamma Alpha Beta]", got)
	}

	fresh := newHarness(t)
	fresh.token = me.token
	if got := fresh.guildOrder(); !same(got, []string{"Gamma", "Alpha", "Beta"}) {
		t.Errorf("a new connection sees %v; the order did not survive the request", got)
	}
}

func TestAServerJoinedLaterGoesToTheBottomOfTheRail(t *testing.T) {
	me := newHarness(t)
	me.registerUser()

	first := me.createGuild("Alpha")
	second := me.createGuild("Beta")
	me.reorder([]uuid.UUID{second.ID, first.ID}, http.StatusOK)

	me.createGuild("Gamma")

	if got := me.guildOrder(); !same(got, []string{"Beta", "Alpha", "Gamma"}) {
		t.Errorf("order = %v, want the new server last rather than jumping to the top", got)
	}
}

func TestReorderingIsRefusedUnlessItNamesEveryServerOnce(t *testing.T) {
	me := newHarness(t)
	me.registerUser()

	first := me.createGuild("Alpha")
	second := me.createGuild("Beta")

	me.reorder([]uuid.UUID{first.ID}, http.StatusBadRequest)
	me.reorder([]uuid.UUID{first.ID, first.ID}, http.StatusBadRequest)
	me.reorder([]uuid.UUID{first.ID, second.ID, uuid.New()}, http.StatusBadRequest)

	if got := me.guildOrder(); !same(got, []string{"Alpha", "Beta"}) {
		t.Errorf("a refused reorder still changed the order: %v", got)
	}
}

func TestYouCannotOrderAServerYouAreNotIn(t *testing.T) {
	me := newHarness(t)
	me.registerUser()
	mine := me.createGuild("Mine")

	stranger := me.newUser()
	theirs := stranger.createGuild("Theirs")

	stranger.reorder([]uuid.UUID{mine.ID}, http.StatusNotFound)

	if got := stranger.guildOrder(); !same(got, []string{"Theirs"}) {
		t.Errorf("stranger's rail = %v, want just their own server", got)
	}
	if got := me.guildOrder(); !same(got, []string{"Mine"}) {
		t.Errorf("my rail was touched by somebody else: %v", got)
	}
	_ = theirs
}

func TestOnePersonsOrderDoesNotMoveAnothers(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	alpha := owner.createGuild("Alpha")
	beta := owner.createGuild("Beta")

	guest := owner.inviteMember(alpha.ID)
	inviteBeta := owner.createInvite(beta.ID, map[string]any{})
	guest.mustDo(http.MethodPost, "/api/v1/invites/"+inviteBeta.Code, http.StatusOK, nil, nil)

	guest.reorder([]uuid.UUID{beta.ID, alpha.ID}, http.StatusOK)

	if got := guest.guildOrder(); !same(got, []string{"Beta", "Alpha"}) {
		t.Errorf("guest order = %v, want [Beta Alpha]", got)
	}
	if got := owner.guildOrder(); !same(got, []string{"Alpha", "Beta"}) {
		t.Errorf("owner order = %v; one person's rail moved another's", got)
	}
}
