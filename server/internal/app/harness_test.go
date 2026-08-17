//go:build e2e

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/app"
	"github.com/esuEdu/go-tauri-discord/internal/config"
	"github.com/esuEdu/go-tauri-discord/internal/db"
	"github.com/esuEdu/go-tauri-discord/internal/platform/pubsub"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const defaultTestDatabaseURL = "postgres://vocalis:vocalis@localhost:5432/vocalis?sslmode=disable"

type harness struct {
	t      *testing.T
	server *httptest.Server
	token  string
	email  string
}

func randomSuffix() string {
	return strings.ReplaceAll(uuid.NewString()[:8], "-", "")
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = defaultTestDatabaseURL
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Skipf("no database at %s (run `make db-up migrate`): %v", url, err)
	}

	cfg := config.Config{
		Env:                "test",
		TrustedProxies:     []string{"127.0.0.1/32", "::1/128"},
		RegisterPerHour:    10,
		LoginPerMinute:     20,
		MessagesPerMinute:  60,
		MaxSessionsPerUser: 5,
		JWTSecret:          []byte("test-secret-that-is-long-enough-to-pass"),
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    24 * time.Hour,
		HeartbeatInterval:  30 * time.Second,
		CORSOrigins:        []string{"*"},
	}

	broker := pubsub.NewMemory()
	application := app.New(cfg, pool, broker)
	server := httptest.NewServer(application.Handler)

	t.Cleanup(func() {
		server.Close()
		application.Close()
		broker.Close()
		pool.Close()
	})

	return &harness{t: t, server: server}
}

func (h *harness) newUser() *harness {
	h.t.Helper()
	other := &harness{t: h.t, server: h.server}
	other.registerUser()
	return other
}

func (h *harness) do(method, path string, body, out any) int {
	h.t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	if out != nil {
		raw, _ := io.ReadAll(resp.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, out); err != nil {
				h.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
			}
		}
	}
	return resp.StatusCode
}

func (h *harness) raw(method, path string, body any) *http.Response {
	h.t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (h *harness) mustDo(method, path string, wantStatus int, body, out any) {
	h.t.Helper()
	if got := h.do(method, path, body, out); got != wantStatus {
		h.t.Fatalf("%s %s = %d, want %d", method, path, got, wantStatus)
	}
}

// registerUser creates a fresh account. The name is randomised so repeated
// runs against the same database do not collide on the unique indexes.
func (h *harness) registerUser() (userID uuid.UUID, refreshToken string) {
	h.t.Helper()

	name := "e2e" + randomSuffix()
	var out struct {
		User   events.User `json:"user"`
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	h.mustDo(http.MethodPost, "/api/v1/auth/register", http.StatusCreated, map[string]string{
		"username": name,
		"email":    name + "@example.test",
		"password": "supersecret1",
	}, &out)

	h.token = out.Tokens.AccessToken
	h.email = name + "@example.test"
	return out.User.ID, out.Tokens.RefreshToken
}

type socket struct {
	t    *testing.T
	conn *websocket.Conn
}

func (h *harness) dial() *socket {
	h.t.Helper()

	url := "ws" + strings.TrimPrefix(h.server.URL, "http") + "/gateway"
	conn, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		h.t.Fatalf("dial gateway: %v", err)
	}
	h.t.Cleanup(func() { conn.CloseNow() })
	return &socket{t: h.t, conn: conn}
}

func (s *socket) read() events.Frame {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	typ, data, err := s.conn.Read(ctx)
	if err != nil {
		s.t.Fatalf("read frame: %v", err)
	}
	if typ != websocket.MessageText {
		s.t.Fatalf("expected a text frame, got %v", typ)
	}
	var frame events.Frame
	if err := json.Unmarshal(data, &frame); err != nil {
		s.t.Fatalf("decode frame %q: %v", data, err)
	}
	return frame
}

// readUntil skips frames that do not match, so an unrelated dispatch such as
// PRESENCE_UPDATE cannot desynchronise an assertion.
func (s *socket) readAny() *events.Frame {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	typ, data, err := s.conn.Read(ctx)
	if err != nil || typ != websocket.MessageText {
		return nil
	}
	var frame events.Frame
	if json.Unmarshal(data, &frame) != nil {
		return nil
	}
	return &frame
}

func (s *socket) quietFor(within time.Duration, match func(events.Frame) bool) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		typ, data, err := s.conn.Read(ctx)
		cancel()
		if err != nil {
			return true
		}
		if typ != websocket.MessageText {
			continue
		}
		var frame events.Frame
		if json.Unmarshal(data, &frame) != nil {
			continue
		}
		if match(frame) {
			return false
		}
	}
	return true
}

func (s *socket) readUntil(what string, match func(events.Frame) bool) events.Frame {
	s.t.Helper()

	for range 20 {
		if frame := s.read(); match(frame) {
			return frame
		}
	}
	s.t.Fatalf("never received %s", what)
	return events.Frame{}
}

func (s *socket) readEvent(t events.EventType) events.Frame {
	s.t.Helper()
	return s.readUntil(string(t), func(f events.Frame) bool { return f.T == t })
}

func (s *socket) write(frame events.Frame) {
	s.t.Helper()

	raw, err := json.Marshal(frame)
	if err != nil {
		s.t.Fatalf("marshal frame: %v", err)
	}
	if err := s.conn.Write(context.Background(), websocket.MessageText, raw); err != nil {
		s.t.Fatalf("write frame: %v", err)
	}
}

func (s *socket) identify(token string) events.Ready {
	s.t.Helper()

	if op := s.read().Op; op != events.OpHello {
		s.t.Fatalf("first frame op = %d, want HELLO (%d)", op, events.OpHello)
	}
	s.write(events.Frame{Op: events.OpIdentify, D: mustJSON(s.t, events.Identify{Token: token})})

	frame := s.read()
	if frame.T != events.EventReady {
		s.t.Fatalf("first dispatch = %q, want READY; presence must not overtake it", frame.T)
	}
	if frame.S != 1 {
		s.t.Errorf("READY sequence = %d, want 1", frame.S)
	}

	var ready events.Ready
	decode(s.t, frame.D, &ready)
	return ready
}

func decode(t *testing.T, raw json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode payload %q: %v", raw, err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
