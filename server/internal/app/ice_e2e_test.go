//go:build e2e

package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/esuEdu/go-tauri-discord/internal/config"
	"github.com/esuEdu/go-tauri-discord/internal/ice"
)

func withRelay(t *testing.T, secret string) func(*config.Config) {
	t.Helper()
	servers, err := ice.ParseServers("stun:stun.example.org:3478,turn:relay.example.org:3478")
	if err != nil {
		t.Fatalf("parse ice servers: %v", err)
	}
	return func(c *config.Config) {
		c.ICEServers = servers
		c.TURNSecret = secret
		c.TURNTTL = time.Hour
	}
}

func TestReadyCarriesTheRelayToUse(t *testing.T) {
	h := newHarness(t, withRelay(t, "shared-with-coturn"))
	h.registerUser()

	ready := h.dial().identify(h.token)

	if len(ready.ICEServers) != 2 {
		t.Fatalf("READY carried %d ice servers, want 2: a client that is told nothing "+
			"falls back to a hardcoded stun server", len(ready.ICEServers))
	}

	relay := ready.ICEServers[1]
	if len(relay.URLs) != 1 || relay.URLs[0] != "turn:relay.example.org:3478" {
		t.Fatalf("second server = %+v, want the turn url", relay)
	}
	if relay.Username == "" || relay.Credential == "" {
		t.Fatal("the turn server arrived without credentials, which is what pion rejects")
	}
	if !strings.HasSuffix(relay.Username, ":"+ready.User.ID.String()) {
		t.Errorf("username %q does not end in this user's id, so it is not theirs alone",
			relay.Username)
	}

	if stun := ready.ICEServers[0]; stun.Username != "" || stun.Credential != "" {
		t.Errorf("the stun server was given credentials: %+v", stun)
	}
}

func TestARelayWithoutASecretIsNotOfferedToTheClient(t *testing.T) {
	h := newHarness(t, withRelay(t, ""))
	h.registerUser()

	ready := h.dial().identify(h.token)

	for _, server := range ready.ICEServers {
		for _, url := range server.URLs {
			if strings.HasPrefix(url, "turn:") {
				t.Errorf("a turn server with no credentials was sent to the client: %+v", server)
			}
		}
	}
}

func TestTwoPeopleGetDifferentRelayCredentials(t *testing.T) {
	h := newHarness(t, withRelay(t, "shared-with-coturn"))
	h.registerUser()
	other := h.newUser()

	mine := h.dial().identify(h.token)
	theirs := other.dial().identify(other.token)

	if mine.ICEServers[1].Credential == theirs.ICEServers[1].Credential {
		t.Error("both people were handed the same relay credential")
	}
}
