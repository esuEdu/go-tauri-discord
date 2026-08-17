package ratelimit

import (
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
)

type KeyFunc func(*http.Request) string

type Rule struct {
	Match   func(*http.Request) bool
	Limiter *Limiter
}

func Method(method, prefix string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Method == method && strings.HasPrefix(r.URL.Path, prefix)
	}
}

func MethodSuffix(method, prefix, suffix string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Method == method &&
			strings.HasPrefix(r.URL.Path, prefix) &&
			strings.HasSuffix(r.URL.Path, suffix)
	}
}

func Any(prefix string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, prefix)
	}
}

func Middleware(key KeyFunc, rules []Rule) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, rule := range rules {
				if !rule.Match(r) {
					continue
				}
				k := key(r)
				if k == "" {
					break
				}
				allowed, retryAfter := rule.Limiter.Allow(k)
				if !allowed {
					seconds := max(int(math.Ceil(retryAfter.Seconds())), 1)
					w.Header().Set("Retry-After", strconv.Itoa(seconds))
					httpx.Error(w, r, domain.RateLimited("too many requests"))
					return
				}
				break
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ParsePrefixes(values []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if p, err := netip.ParsePrefix(raw); err == nil {
			out = append(out, p)
			continue
		}
		if addr, err := netip.ParseAddr(raw); err == nil {
			out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return out
}

func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	peer := peerAddr(r)
	if !peer.IsValid() {
		return r.RemoteAddr
	}
	if !withinAny(peer, trusted) {
		return peer.String()
	}

	if cf, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); err == nil {
		return cf.String()
	}

	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(forwarded[i]))
		if err != nil {
			continue
		}
		if !withinAny(addr, trusted) {
			return addr.String()
		}
	}
	return peer.String()
}

func peerAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func withinAny(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
