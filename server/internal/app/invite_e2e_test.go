//go:build e2e

package app_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type inviteView struct {
	Code        string `json:"code"`
	GuildID     string `json:"guild_id"`
	GuildName   string `json:"guild_name"`
	MemberCount int64  `json:"member_count"`
	Uses        int32  `json:"uses"`
	MaxUses     *int32 `json:"max_uses"`
}

func (h *harness) createInvite(guildID uuid.UUID, body any) inviteView {
	h.t.Helper()
	var out inviteView
	h.mustDo(http.MethodPost, "/api/v1/guilds/"+guildID.String()+"/invites",
		http.StatusCreated, body, &out)
	return out
}

func TestOpenJoinEndpointIsGone(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Private")

	outsider := newHarness(t)
	outsider.registerUser()

	status := outsider.do(http.MethodPut,
		"/api/v1/guilds/"+guild.ID.String()+"/members/@me", nil, nil)
	if status == http.StatusOK || status == http.StatusNoContent {
		t.Fatalf("joining by raw guild id still works (%d)", status)
	}

	var mine []events.Guild
	outsider.mustDo(http.MethodGet, "/api/v1/guilds", http.StatusOK, nil, &mine)
	for _, g := range mine {
		if g.ID == guild.ID {
			t.Error("a non-member holding only the guild id got in")
		}
	}
}

func TestInviteRoundTrip(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Invite Me")
	invite := owner.createInvite(guild.ID, map[string]any{})

	if len(invite.Code) != 8 {
		t.Errorf("code %q is %d chars, want 8", invite.Code, len(invite.Code))
	}
	if invite.GuildName != "Invite Me" || invite.MemberCount != 1 {
		t.Errorf("preview data wrong: %+v", invite)
	}

	anon := newHarness(t)
	var preview inviteView
	anon.mustDo(http.MethodGet, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, &preview)
	if preview.GuildName != "Invite Me" {
		t.Errorf("unauthenticated preview returned %+v", preview)
	}

	friend := newHarness(t)
	friend.registerUser()
	var joined events.Guild
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, &joined)
	if joined.ID != guild.ID {
		t.Fatalf("redeem returned guild %s, want %s", joined.ID, guild.ID)
	}

	channels := friend.listChannels(guild.ID)
	if len(channels) != 2 {
		t.Errorf("joined member sees %d channels, want 2", len(channels))
	}

	text, _ := friend.textAndVoice(guild.ID)
	friend.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "hello from the invite"}, nil)
}

func TestRedeemingTwiceDoesNotSpendASecondUse(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Idempotent")
	one := int32(1)
	invite := owner.createInvite(guild.ID, map[string]any{"max_uses": one})

	friend := newHarness(t)
	friend.registerUser()
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)
	friend.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)
}

func TestExhaustedInviteIsRejected(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("One Shot")
	one := int32(1)
	invite := owner.createInvite(guild.ID, map[string]any{"max_uses": one})

	first := newHarness(t)
	first.registerUser()
	first.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	second := newHarness(t)
	second.registerUser()
	second.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusForbidden, nil, nil)

	anon := newHarness(t)
	anon.mustDo(http.MethodGet, "/api/v1/invites/"+invite.Code, http.StatusNotFound, nil, nil)
}

func TestConcurrentRedemptionAdmitsExactlyOne(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Race")
	one := int32(1)
	invite := owner.createInvite(guild.ID, map[string]any{"max_uses": one})

	const contenders = 8
	clients := make([]*harness, contenders)
	for i := range clients {
		clients[i] = newHarness(t)
		clients[i].registerUser()
	}

	var wg sync.WaitGroup
	results := make([]int, contenders)
	start := make(chan struct{})
	for i, c := range clients {
		wg.Add(1)
		go func(i int, c *harness) {
			defer wg.Done()
			<-start
			results[i] = c.do(http.MethodPost, "/api/v1/invites/"+invite.Code, nil, nil)
		}(i, c)
	}
	close(start)
	wg.Wait()

	admitted := 0
	for _, status := range results {
		if status == http.StatusOK {
			admitted++
		}
	}
	if admitted != 1 {
		t.Fatalf("%d of %d concurrent redeemers admitted, want exactly 1 (results: %v)",
			admitted, contenders, results)
	}

	var members []map[string]any
	owner.mustDo(http.MethodGet, "/api/v1/guilds/"+guild.ID.String()+"/members",
		http.StatusOK, nil, &members)
	if len(members) != 2 {
		t.Errorf("guild has %d members, want 2 (owner plus one redeemer)", len(members))
	}
}

func TestRevokedInviteStopsNewJoinsButKeepsMembers(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Revoke")
	invite := owner.createInvite(guild.ID, map[string]any{})

	early := newHarness(t)
	early.registerUser()
	early.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusOK, nil, nil)

	owner.mustDo(http.MethodDelete, "/api/v1/invites/"+invite.Code, http.StatusNoContent, nil, nil)

	late := newHarness(t)
	late.registerUser()
	late.mustDo(http.MethodPost, "/api/v1/invites/"+invite.Code, http.StatusForbidden, nil, nil)

	channels := early.listChannels(guild.ID)
	if len(channels) != 2 {
		t.Error("revoking the invite removed access for someone who already joined")
	}
}

func TestOutsidersCannotMintOrRevokeInvites(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Guarded")
	invite := owner.createInvite(guild.ID, map[string]any{})

	outsider := newHarness(t)
	outsider.registerUser()

	outsider.mustDo(http.MethodPost, "/api/v1/guilds/"+guild.ID.String()+"/invites",
		http.StatusNotFound, map[string]any{}, nil)
	outsider.mustDo(http.MethodDelete, "/api/v1/invites/"+invite.Code,
		http.StatusNotFound, nil, nil)
	outsider.mustDo(http.MethodGet, "/api/v1/guilds/"+guild.ID.String()+"/invites",
		http.StatusNotFound, nil, nil)
}

func TestUnknownInviteCodes(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	h.mustDo(http.MethodGet, "/api/v1/invites/nosuchco", http.StatusNotFound, nil, nil)
	h.mustDo(http.MethodPost, "/api/v1/invites/nosuchco", http.StatusNotFound, nil, nil)
}
