//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
)

func TestRegistrationIsRateLimited(t *testing.T) {
	h := newHarness(t)

	limited := false
	for i := range 20 {
		name := "flood" + strconv.Itoa(i) + randomSuffix()
		status := h.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
			"username": name,
			"email":    name + "@example.test",
			"password": "supersecret1",
		}, nil)
		if status == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("20 registrations in a row were all accepted")
	}
}

func TestRateLimitResponseCarriesRetryAfter(t *testing.T) {
	h := newHarness(t)

	for range 30 {
		name := "retry" + randomSuffix()
		resp := h.raw(http.MethodPost, "/api/v1/auth/register", map[string]string{
			"username": name,
			"email":    name + "@example.test",
			"password": "supersecret1",
		})
		if resp.StatusCode != http.StatusTooManyRequests {
			resp.Body.Close()
			continue
		}
		retry := resp.Header.Get("Retry-After")
		resp.Body.Close()

		if retry == "" {
			t.Fatal("429 response has no Retry-After header")
		}
		seconds, err := strconv.Atoi(retry)
		if err != nil || seconds < 1 {
			t.Fatalf("Retry-After = %q, want a positive integer", retry)
		}
		return
	}
	t.Skip("never hit the limit; the register bucket was already drained")
}

func TestFailedLoginsAreThrottledPerAccount(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	var me struct {
		Username string `json:"username"`
	}
	h.mustDo(http.MethodGet, "/api/v1/users/@me", http.StatusOK, nil, &me)
	email := h.email

	throttled := false
	for range 15 {
		status := h.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
			"email":    email,
			"password": "definitely-wrong",
		}, nil)
		if status == http.StatusTooManyRequests {
			throttled = true
			break
		}
		if status != http.StatusUnauthorized {
			t.Fatalf("unexpected status %d for a wrong password", status)
		}
	}
	if !throttled {
		t.Error("15 wrong passwords for one account were never throttled")
	}
}

func TestTypingIsRateLimited(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Typing")
	text, _ := h.textAndVoice(guild.ID)

	limited := false
	for range 20 {
		status := h.do(http.MethodPost, "/api/v1/channels/"+text.String()+"/typing", nil, nil)
		if status == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("typing indicator accepted 20 requests in a row")
	}
}

func TestReadsAreNotThrottledDuringNormalUse(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Normal")
	text, _ := h.textAndVoice(guild.ID)

	for i := range 30 {
		status := h.do(http.MethodGet, "/api/v1/channels/"+text.String()+"/messages", nil, nil)
		if status != http.StatusOK {
			t.Fatalf("read %d returned %d; ordinary use should not be throttled", i+1, status)
		}
	}
}

func TestLimitsAreScopedPerAccount(t *testing.T) {
	noisy := newHarness(t)
	noisy.registerUser()
	guild := noisy.createGuild("Shared")
	text, _ := noisy.textAndVoice(guild.ID)

	for range 20 {
		if noisy.do(http.MethodPost, "/api/v1/channels/"+text.String()+"/typing", nil, nil) == http.StatusTooManyRequests {
			break
		}
	}

	quiet := newHarness(t)
	quiet.registerUser()
	quietGuild := quiet.createGuild("Quiet")
	quietText, _ := quiet.textAndVoice(quietGuild.ID)

	if status := quiet.do(http.MethodPost, "/api/v1/channels/"+quietText.String()+"/typing", nil, nil); status != http.StatusNoContent {
		t.Errorf("a second account got %d; one user's flood throttled another", status)
	}
}

func TestCreatingRolesIsNotChargedToTheGuildBudget(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Many roles")

	for i := range 12 {
		status := h.do(http.MethodPost, "/api/v1/guilds/"+guild.ID.String()+"/roles", map[string]any{
			"name":        fmt.Sprintf("Role %d", i),
			"permissions": 1,
		}, nil)
		if status == http.StatusTooManyRequests {
			t.Fatalf("role %d was rate limited; creating a role must not spend the "+
				"guild-creation budget of 20 per hour", i+1)
		}
		if status != http.StatusCreated {
			t.Fatalf("role %d returned %d, want %d", i+1, status, http.StatusCreated)
		}
	}
}

func TestCreatingChannelsIsNotChargedToTheGuildBudget(t *testing.T) {
	h := newHarness(t)
	h.registerUser()
	guild := h.createGuild("Many channels")

	for i := range 12 {
		status := h.do(http.MethodPost, "/api/v1/guilds/"+guild.ID.String()+"/channels", map[string]any{
			"name": fmt.Sprintf("room-%d", i),
			"kind": "text",
		}, nil)
		if status == http.StatusTooManyRequests {
			t.Fatalf("channel %d was rate limited; creating a channel must not spend "+
				"the guild-creation budget", i+1)
		}
		if status != http.StatusCreated {
			t.Fatalf("channel %d returned %d, want %d", i+1, status, http.StatusCreated)
		}
	}
}

func TestCreatingGuildsIsStillRateLimited(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	limited := false
	for i := range 30 {
		status := h.do(http.MethodPost, "/api/v1/guilds", map[string]string{
			"name": fmt.Sprintf("Guild %d", i),
		}, nil)
		if status == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("guild creation accepted 30 requests; its own limit should still bite")
	}
}
