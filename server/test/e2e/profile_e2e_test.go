//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type profileView struct {
	User     events.User   `json:"user"`
	GuildID  uuid.UUID     `json:"guild_id"`
	Nickname *string       `json:"nickname"`
	JoinedAt time.Time     `json:"joined_at"`
	Bio      *string       `json:"bio"`
	Roles    []events.Role `json:"roles"`
}

func profilePath(guildID uuid.UUID, who events.UserID) string {
	return "/api/v1/guilds/" + guildID.String() + "/members/" + who.String() + "/profile"
}

func (h *harness) profile(guildID uuid.UUID, who events.UserID) profileView {
	h.t.Helper()
	var out profileView
	h.mustDo(http.MethodGet, profilePath(guildID, who), http.StatusOK, nil, &out)
	return out
}

func TestAProfileAnswersWhoSomebodyIsHere(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Introductions")

	member := owner.inviteMember(guild.ID)
	memberID, memberName := memberIdentity(t, member)
	member.setStatus(map[string]any{"bio": "  I keep the lights on  "}, http.StatusOK)

	role := owner.createRole(guild.ID, map[string]any{"name": "Keeper", "permissions": 0, "position": 3})
	owner.mustDo(http.MethodPut, memberRolesPath(guild.ID, memberID, role.ID),
		http.StatusNoContent, nil, nil)

	got := owner.profile(guild.ID, memberID)

	if got.User.ID != memberID || got.User.Username != memberName {
		t.Errorf("profile named %+v, want %s", got.User, memberName)
	}
	if got.Bio == nil || *got.Bio != "I keep the lights on" {
		t.Errorf("bio = %v, want it trimmed and carried", got.Bio)
	}
	if got.JoinedAt.IsZero() {
		t.Error("the profile does not say when they joined, which is half of what it is for")
	}
	if time.Since(got.JoinedAt) > time.Hour {
		t.Errorf("joined_at = %s, which is not when this test made them a member", got.JoinedAt)
	}

	var named []string
	for _, r := range got.Roles {
		named = append(named, r.Name)
	}
	if len(got.Roles) == 0 {
		t.Fatal("the profile carries no roles, so the role system stays invisible")
	}
	found := false
	for _, name := range named {
		if name == "Keeper" {
			found = true
		}
	}
	if !found {
		t.Errorf("roles = %v, want Keeper among them", named)
	}
}

func TestAProfileIsRefusedToAStranger(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Members only")
	member := owner.inviteMember(guild.ID)
	memberID := member.whoAmI()

	stranger := owner.newUser()
	stranger.mustDo(http.MethodGet, profilePath(guild.ID, memberID), http.StatusNotFound, nil, nil)
}

func TestABioIsRefusedWhenItRunsLong(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	long := make([]byte, 501)
	for i := range long {
		long[i] = 'b'
	}
	h.setStatus(map[string]any{"bio": string(long)}, http.StatusBadRequest)
}
