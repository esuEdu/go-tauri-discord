package gateway

import (
	"strconv"
	"sync"
	"time"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/google/uuid"
)

const (
	sendBuffer   = 256
	replaySize   = 256
	resumeWindow = 90 * time.Second
)

type replayEntry struct {
	seq  int64
	data []byte
}

type session struct {
	id     string
	userID uuid.UUID
	user   dbgen.User
	send   chan []byte

	dead      chan struct{}
	closeOnce sync.Once

	mu        sync.Mutex
	seq       int64
	replay    []replayEntry
	topics    map[string]struct{}
	hidden    map[uuid.UUID]uuid.UUID
	connected bool
	expiry    *time.Timer
}

func (s *session) hideInGuild(guildID uuid.UUID, channelIDs []uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hidden == nil {
		s.hidden = make(map[uuid.UUID]uuid.UUID, len(channelIDs))
	}
	for channelID, owner := range s.hidden {
		if owner == guildID {
			delete(s.hidden, channelID)
		}
	}
	for _, channelID := range channelIDs {
		s.hidden[channelID] = guildID
	}
}

func (s *session) canSee(channelID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, hidden := s.hidden[channelID]
	return !hidden
}

func newSession(user dbgen.User) *session {
	return &session{
		id:        uuid.NewString(),
		userID:    user.ID,
		user:      user,
		send:      make(chan []byte, sendBuffer),
		dead:      make(chan struct{}),
		replay:    make([]replayEntry, 0, replaySize),
		topics:    make(map[string]struct{}),
		connected: true,
	}
}

func (s *session) enqueue(raw []byte) {
	s.mu.Lock()
	s.seq++
	framed := withSeq(raw, s.seq)
	if len(s.replay) == replaySize {
		copy(s.replay, s.replay[1:])
		s.replay = s.replay[:replaySize-1]
	}
	s.replay = append(s.replay, replayEntry{seq: s.seq, data: framed})
	s.mu.Unlock()

	select {
	case s.send <- framed:
	default:
		s.kill()
	}
}

func (s *session) enqueueControl(raw []byte) {
	select {
	case s.send <- raw:
	default:
		s.kill()
	}
}

func (s *session) drainQueued() {
	for {
		select {
		case <-s.send:
		default:
			return
		}
	}
}

func (s *session) stopExpiry() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
}

func (s *session) kill() {
	s.closeOnce.Do(func() { close(s.dead) })
}

func (s *session) replayAfter(seq int64) (frames [][]byte, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.replay) == 0 {
		return nil, seq == s.seq
	}
	if seq < s.replay[0].seq-1 || seq > s.seq {
		return nil, false
	}
	for _, e := range s.replay {
		if e.seq > seq {
			frames = append(frames, e.data)
		}
	}
	return frames, true
}

func withSeq(raw []byte, seq int64) []byte {
	if len(raw) < 2 || raw[0] != '{' {
		return raw
	}
	out := make([]byte, 0, len(raw)+16)
	out = append(out, '{', '"', 's', '"', ':')
	out = strconv.AppendInt(out, seq, 10)
	if raw[1] != '}' {
		out = append(out, ',')
	}
	return append(out, raw[1:]...)
}
