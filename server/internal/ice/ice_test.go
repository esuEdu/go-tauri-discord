package ice

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseServers(t *testing.T) {
	servers, err := ParseServers(
		"stun:stun.example.org:3478, turn:relay.example.org:3478|alice|s3cret ,")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("parsed %d servers, want 2 (the trailing empty entry is not one)", len(servers))
	}
	if servers[0].URL != "stun:stun.example.org:3478" || servers[0].Username != "" {
		t.Errorf("bare url parsed as %+v", servers[0])
	}
	if servers[1].Username != "alice" || servers[1].Credential != "s3cret" {
		t.Errorf("credentialled url parsed as %+v", servers[1])
	}
	if servers[0].Relay() || !servers[1].Relay() {
		t.Error("Relay() did not tell stun and turn apart")
	}
}

func TestParseServersRejectsHalfCredentials(t *testing.T) {
	for _, spec := range []string{
		"turn:relay.example.org|alice",
		"turn:relay.example.org|alice|",
		"turn:relay.example.org|a|b|c",
	} {
		if _, err := ParseServers(spec); err == nil {
			t.Errorf("%q was accepted; a half-written credential must not reach pion", spec)
		}
	}
}

func TestMintedCredentialsFollowTheTurnRestConvention(t *testing.T) {
	servers, err := ParseServers("stun:stun.example.org,turn:relay.example.org")
	if err != nil {
		t.Fatal(err)
	}

	const secret = "shared-with-coturn"
	minter := NewMinter(servers, secret, time.Hour)

	minted := minter.For("01a03000-0000-7000-8000-000000000000")
	if len(minted) != 2 {
		t.Fatalf("minted %d servers, want 2", len(minted))
	}

	if minted[0].Username != "" || minted[0].Credential != "" {
		t.Errorf("stun server was given credentials: %+v", minted[0])
	}

	relay := minted[1]
	expiry, identity, found := strings.Cut(relay.Username, ":")
	if !found {
		t.Fatalf("username %q is not <expiry>:<identity>", relay.Username)
	}
	if identity != "01a03000-0000-7000-8000-000000000000" {
		t.Errorf("username carried identity %q", identity)
	}

	seconds, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		t.Fatalf("expiry %q is not a unix timestamp", expiry)
	}
	if left := time.Until(time.Unix(seconds, 0)); left < 55*time.Minute || left > time.Hour {
		t.Errorf("credential expires in %s, want about an hour", left)
	}

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(relay.Username))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if relay.Credential != want {
		t.Errorf("credential = %q, want base64 HMAC-SHA1 of the username", relay.Credential)
	}
}

func TestEveryIdentityGetsItsOwnCredential(t *testing.T) {
	servers, _ := ParseServers("turn:relay.example.org")
	minter := NewMinter(servers, "shared-with-coturn", time.Hour)

	mine := minter.For("someone")
	theirs := minter.For("somebody-else")
	if mine[0].Credential == theirs[0].Credential {
		t.Error("two people were handed the same credential")
	}
}

func TestARelayWithoutASecretIsDroppedAndReported(t *testing.T) {
	servers, _ := ParseServers("stun:stun.example.org,turn:relay.example.org")
	minter := NewMinter(servers, "", time.Hour)

	minted := minter.For("someone")
	if len(minted) != 1 || minted[0].Relay() {
		t.Errorf("a credential-less turn server was handed out: %+v", minted)
	}

	stranded := minter.Unusable()
	if len(stranded) != 1 || stranded[0] != "turn:relay.example.org" {
		t.Errorf("Unusable() = %v, want the turn server so the operator is told", stranded)
	}
}

func TestStaticCredentialsSurviveWithoutASecret(t *testing.T) {
	servers, _ := ParseServers("turn:relay.example.org|alice|s3cret")
	minter := NewMinter(servers, "", time.Hour)

	minted := minter.For("someone")
	if len(minted) != 1 || minted[0].Username != "alice" || minted[0].Credential != "s3cret" {
		t.Errorf("static credentials were not passed through: %+v", minted)
	}
	if len(minter.Unusable()) != 0 {
		t.Error("a server with its own credentials was reported unusable")
	}
}
