package ratelimit

import (
	"net/http"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func newTestLimiter(p Policy, clock *time.Time) *Limiter {
	l := New(p)
	l.Stop()
	l.now = func() time.Time { return *clock }
	return l
}

func TestBurstThenDeny(t *testing.T) {
	now := time.Now()
	l := newTestLimiter(Policy{Every: time.Second, Burst: 3}, &now)

	for i := range 3 {
		if allowed, _ := l.Allow("k"); !allowed {
			t.Fatalf("request %d denied inside the burst", i+1)
		}
	}
	allowed, retryAfter := l.Allow("k")
	if allowed {
		t.Fatal("fourth request allowed past a burst of 3")
	}
	if retryAfter <= 0 || retryAfter > time.Second {
		t.Errorf("retryAfter = %v, want between 0 and 1s", retryAfter)
	}
}

func TestTokensRefillOverTime(t *testing.T) {
	now := time.Now()
	l := newTestLimiter(Policy{Every: time.Second, Burst: 1}, &now)

	if allowed, _ := l.Allow("k"); !allowed {
		t.Fatal("first request denied")
	}
	if allowed, _ := l.Allow("k"); allowed {
		t.Fatal("second immediate request allowed")
	}

	now = now.Add(time.Second)
	if allowed, _ := l.Allow("k"); !allowed {
		t.Error("request denied after a full refill interval")
	}
}

func TestRefillIsCappedAtBurst(t *testing.T) {
	now := time.Now()
	l := newTestLimiter(Policy{Every: time.Second, Burst: 2}, &now)

	l.Allow("k")
	now = now.Add(time.Hour)

	allowed := 0
	for range 10 {
		if ok, _ := l.Allow("k"); ok {
			allowed++
		}
	}
	if allowed != 2 {
		t.Errorf("an hour idle granted %d requests, want the burst of 2", allowed)
	}
}

func TestKeysAreIndependent(t *testing.T) {
	now := time.Now()
	l := newTestLimiter(Policy{Every: time.Second, Burst: 1}, &now)

	l.Allow("alice")
	if allowed, _ := l.Allow("bob"); !allowed {
		t.Error("bob was denied because alice spent her token")
	}
}

func TestEvictionDropsIdleKeys(t *testing.T) {
	now := time.Now()
	l := newTestLimiter(Policy{Every: time.Second, Burst: 1}, &now)

	l.Allow("gone")
	l.Allow("alsogone")
	if l.Size() != 2 {
		t.Fatalf("size = %d, want 2", l.Size())
	}

	now = now.Add(time.Hour)
	l.evictFull()
	if l.Size() != 0 {
		t.Errorf("size = %d after eviction, want 0", l.Size())
	}
}

func TestEvictionKeepsThrottledKeys(t *testing.T) {
	now := time.Now()
	l := newTestLimiter(Policy{Every: time.Hour, Burst: 1}, &now)

	l.Allow("busy")
	l.evictFull()
	if l.Size() != 1 {
		t.Error("a key still owing tokens was evicted, resetting its limit")
	}
}

func TestConcurrentAllowIsRaceFree(t *testing.T) {
	l := New(Policy{Every: time.Millisecond, Burst: 100})
	defer l.Stop()

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				l.Allow("shared")
			}
		}()
	}
	wg.Wait()
}

func request(remote string, headers map[string]string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestClientIPIgnoresHeadersFromUntrustedPeers(t *testing.T) {
	trusted := ParsePrefixes([]string{"127.0.0.1/32"})

	got := ClientIP(request("203.0.113.9:5555", map[string]string{
		"X-Forwarded-For":  "1.2.3.4",
		"CF-Connecting-IP": "5.6.7.8",
	}), trusted)

	if got != "203.0.113.9" {
		t.Errorf("ClientIP = %q, want the peer address; a spoofed header won", got)
	}
}

func TestClientIPUsesForwardedHeaderFromTrustedProxy(t *testing.T) {
	trusted := ParsePrefixes([]string{"127.0.0.1/32"})

	got := ClientIP(request("127.0.0.1:5555", map[string]string{
		"X-Forwarded-For": "198.51.100.7",
	}), trusted)

	if got != "198.51.100.7" {
		t.Errorf("ClientIP = %q, want the forwarded client address", got)
	}
}

func TestClientIPPrefersCloudflareHeader(t *testing.T) {
	trusted := ParsePrefixes([]string{"127.0.0.1/32"})

	got := ClientIP(request("127.0.0.1:5555", map[string]string{
		"CF-Connecting-IP": "198.51.100.7",
		"X-Forwarded-For":  "10.0.0.1",
	}), trusted)

	if got != "198.51.100.7" {
		t.Errorf("ClientIP = %q, want the Cloudflare client address", got)
	}
}

func TestClientIPSkipsTrustedHopsInForwardedChain(t *testing.T) {
	trusted := ParsePrefixes([]string{"127.0.0.1/32", "10.0.0.0/8"})

	got := ClientIP(request("127.0.0.1:5555", map[string]string{
		"X-Forwarded-For": "198.51.100.7, 10.0.0.4, 10.0.0.5",
	}), trusted)

	if got != "198.51.100.7" {
		t.Errorf("ClientIP = %q, want the first untrusted hop", got)
	}
}

func TestClientIPWithNoTrustedProxiesUsesPeer(t *testing.T) {
	got := ClientIP(request("127.0.0.1:5555", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	}), nil)

	if got != "127.0.0.1" {
		t.Errorf("ClientIP = %q, want the peer when nothing is trusted", got)
	}
}

func TestParsePrefixesAcceptsBareAddresses(t *testing.T) {
	got := ParsePrefixes([]string{"127.0.0.1", "10.0.0.0/8", "", "nonsense"})
	if len(got) != 2 {
		t.Fatalf("parsed %d prefixes, want 2: %v", len(got), got)
	}
	if !got[0].Contains(netip.MustParseAddr("127.0.0.1")) {
		t.Error("bare address did not become a host prefix")
	}
}
