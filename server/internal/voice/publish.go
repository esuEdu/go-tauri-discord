package voice

import (
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type PublishSignaler interface {
	ScreenAnswer(userID uuid.UUID, sdp webrtc.SessionDescription)
	ScreenCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit)
}

type publisher struct {
	pc     *webrtc.PeerConnection
	userID uuid.UUID

	mu      sync.Mutex
	layers  map[string]bool
	pending []webrtc.ICECandidateInit
	ready   bool
}

func (s *SFU) AttachPublishSignaler(signaler PublishSignaler) {
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	s.publishSignaler = signaler
}

func (s *SFU) PublishScreen(userID uuid.UUID, offer webrtc.SessionDescription) error {
	s.publishMu.Lock()
	signaler := s.publishSignaler
	if existing := s.publishers[userID]; existing != nil {
		delete(s.publishers, userID)
		delete(s.early, userID)
		existing.pc.Close()
	}
	s.publishMu.Unlock()

	if signaler == nil {
		return ErrNotConnected
	}

	if _, owner := s.roomFor(userID); owner == nil || !owner.mayStream {
		return ErrNotAllowed
	}

	pc, err := s.api.NewPeerConnection(s.config)
	if err != nil {
		return err
	}

	p := &publisher{pc: pc, userID: userID, layers: make(map[string]bool)}

	s.publishMu.Lock()
	s.publishers[userID] = p
	p.pending = append(p.pending, s.early[userID]...)
	delete(s.early, userID)
	s.publishMu.Unlock()

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		signaler.ScreenCandidate(userID, c.ToJSON())
	})

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		p.sawLayer(remote)

		source := SourceScreenAudio
		rid := ""
		if remote.Kind() == webrtc.RTPCodecTypeVideo {
			source = SourceScreen
			rid = remote.RID()
			if rid == "" {
				rid = DefaultLayer
			}
		}

		r, owner := s.roomFor(userID)
		if r == nil || owner == nil {
			slog.Warn("voice: published screen has nowhere to go",
				"user_id", userID, "in_room", r != nil, "has_peer", owner != nil)
			return
		}
		s.forwardLayer(r, owner, pc, remote, source, rid)
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return err
	}

	if err := p.drainCandidates(); err != nil {
		slog.Warn("voice: screen candidate", "user_id", userID, "error", err)
	}

	signaler.ScreenAnswer(userID, answer)
	return nil
}

func (p *publisher) drainCandidates() error {
	p.mu.Lock()
	pending := p.pending
	p.pending = nil
	p.ready = true
	p.mu.Unlock()

	for _, candidate := range pending {
		if err := p.pc.AddICECandidate(candidate); err != nil {
			return err
		}
	}
	return nil
}

func (s *SFU) PublishCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit) error {
	s.publishMu.Lock()
	p := s.publishers[userID]
	if p == nil {
		s.early[userID] = append(s.early[userID], candidate)
		s.publishMu.Unlock()
		return nil
	}
	s.publishMu.Unlock()

	p.mu.Lock()
	if !p.ready {
		p.pending = append(p.pending, candidate)
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	return p.pc.AddICECandidate(candidate)
}

func (s *SFU) PublishedLayers(userID uuid.UUID) []string {
	s.publishMu.Lock()
	p := s.publishers[userID]
	s.publishMu.Unlock()

	if p == nil {
		return nil
	}
	return p.knownLayers()
}

func (s *SFU) StopPublishing(userID uuid.UUID) {
	s.publishMu.Lock()
	p := s.publishers[userID]
	delete(s.publishers, userID)
	delete(s.early, userID)
	s.publishMu.Unlock()

	if p != nil {
		p.pc.Close()
	}
}

func (p *publisher) sawLayer(remote *webrtc.TrackRemote) {
	rid := remote.RID()
	if rid == "" {
		rid = "(none)"
	}

	p.mu.Lock()
	fresh := !p.layers[rid]
	p.layers[rid] = true
	count := len(p.layers)
	p.mu.Unlock()

	if !fresh {
		return
	}
	slog.Info("voice: screen layer arrived",
		"user_id", p.userID,
		"rid", rid,
		"codec", remote.Codec().MimeType,
		"layers_so_far", count)
}

func (p *publisher) knownLayers() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]string, 0, len(p.layers))
	for rid := range p.layers {
		out = append(out, rid)
	}
	return out
}
