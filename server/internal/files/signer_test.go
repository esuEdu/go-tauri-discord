package files

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func partsOf(t *testing.T, signed string) (path, exp, sig string) {
	t.Helper()
	raw, query, found := strings.Cut(signed, "?")
	if !found {
		t.Fatalf("signed url %q carries no query", signed)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	return raw, values.Get("exp"), values.Get("sig")
}

func TestASignedURLIsAccepted(t *testing.T) {
	signer := NewSigner([]byte("a-jwt-secret-long-enough-to-pass"), time.Hour)

	path, exp, sig := partsOf(t, signer.SignedURL("/api/v1/attachments/abc"))
	if !signer.Allows(path, exp, sig) {
		t.Error("a url this signer just produced was rejected")
	}
}

func TestASignatureDoesNotTravelToAnotherFile(t *testing.T) {
	signer := NewSigner([]byte("a-jwt-secret-long-enough-to-pass"), time.Hour)

	_, exp, sig := partsOf(t, signer.SignedURL("/api/v1/attachments/mine"))
	if signer.Allows("/api/v1/attachments/somebody-elses", exp, sig) {
		t.Error("a signature for one attachment unlocked another")
	}
}

func TestAZeroTTLDoesNotMintDeadURLs(t *testing.T) {
	signer := NewSigner([]byte("a-jwt-secret-long-enough-to-pass"), 0)

	path, exp, sig := partsOf(t, signer.SignedURL("/api/v1/attachments/abc"))
	if !signer.Allows(path, exp, sig) {
		t.Error("an unset TTL signed a url that was already expired")
	}
}

func TestAnExpiredSignatureIsRefused(t *testing.T) {
	signer := &Signer{key: []byte("k"), ttl: -time.Second}

	path, exp, sig := partsOf(t, signer.SignedURL("/api/v1/attachments/abc"))
	if signer.Allows(path, exp, sig) {
		t.Error("an already-expired url was accepted")
	}
}

func TestTheExpiryCannotBeMovedWithoutTheKey(t *testing.T) {
	signer := &Signer{key: []byte("k"), ttl: -time.Second}

	path, _, sig := partsOf(t, signer.SignedURL("/api/v1/attachments/abc"))
	later := time.Now().Add(time.Hour).Unix()
	if signer.Allows(path, string(rune(later)), sig) {
		t.Error("pushing the expiry out was enough to revive a dead url")
	}
}

func TestAnotherSecretCannotSign(t *testing.T) {
	mine := NewSigner([]byte("a-jwt-secret-long-enough-to-pass"), time.Hour)
	theirs := NewSigner([]byte("a-different-secret-of-good-length"), time.Hour)

	path, exp, sig := partsOf(t, theirs.SignedURL("/api/v1/attachments/abc"))
	if mine.Allows(path, exp, sig) {
		t.Error("a url signed with a different secret was accepted")
	}
}

func TestGarbageIsRefusedRatherThanPanicking(t *testing.T) {
	signer := NewSigner([]byte("a-jwt-secret-long-enough-to-pass"), time.Hour)

	for _, c := range [][2]string{{"", ""}, {"not-a-number", "x"}, {"99999999999999999999", "x"}} {
		if signer.Allows("/api/v1/attachments/abc", c[0], c[1]) {
			t.Errorf("Allows accepted exp=%q sig=%q", c[0], c[1])
		}
	}
}
