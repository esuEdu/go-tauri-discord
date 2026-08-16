package gateway

import (
	"encoding/json"
	"testing"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/google/uuid"
)

func newTestSession() *session {
	return newSession(dbgen.User{ID: uuid.New(), Username: "tester"})
}

func TestWithSeqProducesValidJSON(t *testing.T) {
	tests := []string{
		`{"op":0,"t":"MESSAGE_CREATE","d":{"content":"hi"}}`,
		`{}`,
		`{"op":4}`,
	}
	for _, in := range tests {
		out := withSeq([]byte(in), 42)

		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("withSeq(%s) produced invalid JSON %s: %v", in, out, err)
		}
		if got["s"] != float64(42) {
			t.Errorf("withSeq(%s) = %s, want s=42", in, out)
		}

		var original map[string]any
		_ = json.Unmarshal([]byte(in), &original)
		for k, v := range original {
			if got[k] == nil && v != nil {
				t.Errorf("withSeq dropped key %q from %s", k, in)
			}
		}
	}
}

func TestEnqueueAssignsMonotonicSequence(t *testing.T) {
	s := newTestSession()
	for range 3 {
		s.enqueue([]byte(`{"op":0}`))
	}

	for want := 1; want <= 3; want++ {
		var frame struct {
			S int64 `json:"s"`
		}
		if err := json.Unmarshal(<-s.send, &frame); err != nil {
			t.Fatal(err)
		}
		if frame.S != int64(want) {
			t.Errorf("sequence = %d, want %d", frame.S, want)
		}
	}
}

func TestReplayAfterReturnsOnlyNewerFrames(t *testing.T) {
	s := newTestSession()
	for range 5 {
		s.enqueue([]byte(`{"op":0}`))
	}

	frames, ok := s.replayAfter(3)
	if !ok {
		t.Fatal("replay should be possible within the buffer")
	}
	if len(frames) != 2 {
		t.Fatalf("replayed %d frames, want 2 (sequences 4 and 5)", len(frames))
	}
}

func TestReplayAfterRejectsAGapLargerThanTheBuffer(t *testing.T) {
	s := newTestSession()
	for range replaySize + 10 {
		s.enqueue([]byte(`{"op":0}`))
	}

	if _, ok := s.replayAfter(1); ok {
		t.Error("a client this far behind must be told to re-identify")
	}

	if _, ok := s.replayAfter(replaySize + 10); !ok {
		t.Error("a fully caught-up client must be resumable")
	}
}

func TestReplayBufferIsBounded(t *testing.T) {
	s := newTestSession()
	for range replaySize * 2 {
		s.enqueue([]byte(`{"op":0}`))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.replay) > replaySize {
		t.Errorf("replay buffer grew to %d, want at most %d", len(s.replay), replaySize)
	}
}

func TestSlowClientIsKilledRatherThanBlocking(t *testing.T) {

	s := newTestSession()
	for range sendBuffer + 1 {
		s.enqueue([]byte(`{"op":0}`))
	}

	select {
	case <-s.dead:
	default:
		t.Error("session should have been killed once its buffer overflowed")
	}
}

func TestDrainQueuedEmptiesTheBuffer(t *testing.T) {
	s := newTestSession()
	for range 10 {
		s.enqueue([]byte(`{"op":0}`))
	}
	s.drainQueued()

	if n := len(s.send); n != 0 {
		t.Errorf("%d frames left queued after drain", n)
	}

	if frames, ok := s.replayAfter(0); !ok || len(frames) != 10 {
		t.Errorf("drain damaged the replay buffer: got %d frames, ok=%v", len(frames), ok)
	}
}
