package ice

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	URL        string
	Username   string
	Credential string
}

func (s Server) Relay() bool {
	return strings.HasPrefix(s.URL, "turn:") || strings.HasPrefix(s.URL, "turns:")
}

func ParseServers(spec string) ([]Server, error) {
	servers := make([]Server, 0)
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		switch len(parts) {
		case 1:
			servers = append(servers, Server{URL: parts[0]})
		case 3:
			if parts[1] == "" || parts[2] == "" {
				return nil, fmt.Errorf("ice server %q has an empty username or credential", parts[0])
			}
			servers = append(servers, Server{URL: parts[0], Username: parts[1], Credential: parts[2]})
		default:
			return nil, fmt.Errorf(
				"ice server %q must be a url, or url|username|credential", entry)
		}
	}
	return servers, nil
}

type Minter struct {
	servers []Server
	secret  []byte
	ttl     time.Duration
}

func NewMinter(servers []Server, secret string, ttl time.Duration) *Minter {
	return &Minter{servers: servers, secret: []byte(secret), ttl: ttl}
}

func (m *Minter) Unusable() []string {
	if m == nil {
		return nil
	}
	stranded := make([]string, 0)
	for _, s := range m.servers {
		if s.Relay() && s.Username == "" && len(m.secret) == 0 {
			stranded = append(stranded, s.URL)
		}
	}
	return stranded
}

func (m *Minter) For(identity string) []Server {
	if m == nil {
		return nil
	}

	out := make([]Server, 0, len(m.servers))
	for _, s := range m.servers {
		if !s.Relay() || s.Username != "" {
			out = append(out, s)
			continue
		}
		if len(m.secret) == 0 {
			continue
		}
		username, credential := m.credentials(identity)
		out = append(out, Server{URL: s.URL, Username: username, Credential: credential})
	}
	return out
}

func (m *Minter) credentials(identity string) (string, string) {
	expiry := time.Now().Add(m.ttl).Unix()
	username := strconv.FormatInt(expiry, 10) + ":" + identity

	mac := hmac.New(sha1.New, m.secret)
	mac.Write([]byte(username))
	return username, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
