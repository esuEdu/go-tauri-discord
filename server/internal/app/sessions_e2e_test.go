//go:build e2e

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func TestAReconnectingClientIsNotLockedOutByItsOwnGhosts(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	for attempt := range 7 {
		sock := h.dial()
		ready := sock.identify(h.token)
		if ready.User.ID.String() == "" {
			t.Fatalf("reconnect %d did not produce a READY", attempt+1)
		}
		_ = sock.conn.Close(websocket.StatusNormalClosure, "gone")
	}
}

func TestLiveConnectionsAreStillTheCeiling(t *testing.T) {
	h := newHarness(t)
	h.registerUser()

	held := make([]*socket, 0, 5)
	for range 5 {
		sock := h.dial()
		sock.identify(h.token)
		held = append(held, sock)
	}
	defer func() {
		for _, s := range held {
			_ = s.conn.Close(websocket.StatusNormalClosure, "")
		}
	}()

	reason := identifyRejected(t, h)
	if !strings.Contains(reason, "too many concurrent sessions") {
		t.Errorf("the sixth connection was refused with %q, want the session-limit reason", reason)
	}
}

func identifyRejected(t *testing.T, h *harness) string {
	t.Helper()

	sock := h.dial()
	defer sock.conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var hello events.Frame
	if _, raw, err := sock.conn.Read(ctx); err != nil {
		t.Fatalf("no HELLO arrived: %v", err)
	} else if err := json.Unmarshal(raw, &hello); err != nil || hello.Op != events.OpHello {
		t.Fatalf("first frame was not HELLO")
	}

	identify, err := json.Marshal(events.Frame{
		Op: events.OpIdentify, D: mustJSON(t, events.Identify{Token: h.token}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sock.conn.Write(ctx, websocket.MessageText, identify); err != nil {
		t.Fatalf("could not send IDENTIFY: %v", err)
	}

	if _, _, err := sock.conn.Read(ctx); err == nil {
		t.Fatal("a connection past the live limit was admitted")
	} else {
		return websocket.CloseStatus(err).String() + ": " + err.Error()
	}
	return ""
}
