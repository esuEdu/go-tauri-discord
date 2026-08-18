package voice

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

var ErrNotConnected = errors.New("voice: peer is not connected")

type Signaler interface {
	SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription)
	SendCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit)
	VoiceClosed(userID uuid.UUID)
}

type SFU struct {
	api      *webrtc.API
	config   webrtc.Configuration
	signaler Signaler

	mu    sync.Mutex
	rooms map[uuid.UUID]*room
	homes map[uuid.UUID]uuid.UUID
}

type room struct {
	channelID uuid.UUID
	peers     map[uuid.UUID]*peer
	tracks    map[string]*webrtc.TrackLocalStaticRTP
}

type peer struct {
	userID uuid.UUID
	pc     *webrtc.PeerConnection
	owned  map[string]bool

	mu         sync.Mutex
	pending    []webrtc.ICECandidateInit
	haveRemote bool
}

const (
	signalAttempts = 25
	signalBackoff  = 2 * time.Second
	rtpBufferSize  = 1500
)

func New(signaler Signaler, iceServers []string) (*SFU, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	registry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, registry); err != nil {
		return nil, err
	}

	servers := make([]webrtc.ICEServer, 0, len(iceServers))
	for _, url := range iceServers {
		if url != "" {
			servers = append(servers, webrtc.ICEServer{URLs: []string{url}})
		}
	}

	return &SFU{
		api: webrtc.NewAPI(
			webrtc.WithMediaEngine(mediaEngine),
			webrtc.WithInterceptorRegistry(registry),
		),
		config:   webrtc.Configuration{ICEServers: servers},
		signaler: signaler,
		rooms:    make(map[uuid.UUID]*room),
		homes:    make(map[uuid.UUID]uuid.UUID),
	}, nil
}

func (s *SFU) Join(channelID, userID uuid.UUID) error {
	s.Leave(userID)

	pc, err := s.api.NewPeerConnection(s.config)
	if err != nil {
		return err
	}
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		pc.Close()
		return err
	}

	p := &peer{userID: userID, pc: pc, owned: make(map[string]bool)}

	s.mu.Lock()
	r, ok := s.rooms[channelID]
	if !ok {
		r = &room{
			channelID: channelID,
			peers:     make(map[uuid.UUID]*peer),
			tracks:    make(map[string]*webrtc.TrackLocalStaticRTP),
		}
		s.rooms[channelID] = r
	}
	r.peers[userID] = p
	s.homes[userID] = channelID
	s.mu.Unlock()

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		s.signaler.SendCandidate(userID, c.ToJSON())
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			s.Leave(userID)
		}
	})

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		s.forward(r, p, remote)
	})

	s.mu.Lock()
	s.signalLocked(r)
	s.mu.Unlock()
	return nil
}

func (s *SFU) forward(r *room, p *peer, remote *webrtc.TrackRemote) {
	local, err := webrtc.NewTrackLocalStaticRTP(
		remote.Codec().RTPCodecCapability, remote.ID(), remote.StreamID())
	if err != nil {
		slog.Error("voice: create local track", "error", err)
		return
	}

	s.mu.Lock()
	r.tracks[local.ID()] = local
	p.owned[local.ID()] = true
	s.signalLocked(r)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(r.tracks, local.ID())
		delete(p.owned, local.ID())
		s.signalLocked(r)
		s.mu.Unlock()
	}()

	buf := make([]byte, rtpBufferSize)
	for {
		n, _, err := remote.Read(buf)
		if err != nil {
			return
		}
		if _, err := local.Write(buf[:n]); err != nil {
			return
		}
	}
}

func (s *SFU) Leave(userID uuid.UUID) {
	s.mu.Lock()
	channelID, ok := s.homes[userID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.homes, userID)

	r := s.rooms[channelID]
	if r == nil {
		s.mu.Unlock()
		return
	}
	p := r.peers[userID]
	delete(r.peers, userID)

	if p != nil {
		for id := range p.owned {
			delete(r.tracks, id)
		}
	}
	empty := len(r.peers) == 0
	if empty {
		delete(s.rooms, channelID)
	} else {
		s.signalLocked(r)
	}
	s.mu.Unlock()

	if p != nil {
		p.pc.Close()
	}
}

func (s *SFU) Answer(userID uuid.UUID, sdp webrtc.SessionDescription) error {
	p := s.peerFor(userID)
	if p == nil {
		return ErrNotConnected
	}
	if err := p.pc.SetRemoteDescription(sdp); err != nil {
		return err
	}
	return p.drainCandidates()
}

func (s *SFU) AddCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit) error {
	p := s.peerFor(userID)
	if p == nil {
		return ErrNotConnected
	}

	p.mu.Lock()
	if !p.haveRemote {
		p.pending = append(p.pending, candidate)
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()
	return p.pc.AddICECandidate(candidate)
}

func (s *SFU) ChannelOf(userID uuid.UUID) (uuid.UUID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channelID, ok := s.homes[userID]
	return channelID, ok
}

func (s *SFU) Participants(channelID uuid.UUID) []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := s.rooms[channelID]
	if r == nil {
		return nil
	}
	out := make([]uuid.UUID, 0, len(r.peers))
	for id := range r.peers {
		out = append(out, id)
	}
	return out
}

func (s *SFU) Close() {
	s.mu.Lock()
	peers := make([]*peer, 0)
	for _, r := range s.rooms {
		for _, p := range r.peers {
			peers = append(peers, p)
		}
	}
	s.rooms = make(map[uuid.UUID]*room)
	s.homes = make(map[uuid.UUID]uuid.UUID)
	s.mu.Unlock()

	for _, p := range peers {
		p.pc.Close()
	}
}

func (s *SFU) peerFor(userID uuid.UUID) *peer {
	s.mu.Lock()
	defer s.mu.Unlock()

	channelID, ok := s.homes[userID]
	if !ok {
		return nil
	}
	r := s.rooms[channelID]
	if r == nil {
		return nil
	}
	return r.peers[userID]
}

func (p *peer) drainCandidates() error {
	p.mu.Lock()
	pending := p.pending
	p.pending = nil
	p.haveRemote = true
	p.mu.Unlock()

	for _, c := range pending {
		if err := p.pc.AddICECandidate(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *SFU) signalLocked(r *room) {
	for attempt := range signalAttempts {
		if s.syncLocked(r) {
			return
		}
		if attempt == signalAttempts-1 {
			go func() {
				time.Sleep(signalBackoff)
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.rooms[r.channelID] == r {
					s.signalLocked(r)
				}
			}()
		}
	}
}

func (s *SFU) syncLocked(r *room) bool {
	for userID, p := range r.peers {
		if p.pc.ConnectionState() == webrtc.PeerConnectionStateClosed {
			delete(r.peers, userID)
			delete(s.homes, userID)
			continue
		}

		sent := map[string]bool{}
		for _, sender := range p.pc.GetSenders() {
			track := sender.Track()
			if track == nil {
				continue
			}
			if _, live := r.tracks[track.ID()]; !live {
				if err := p.pc.RemoveTrack(sender); err != nil {
					return false
				}
				continue
			}
			sent[track.ID()] = true
		}

		changed := false
		for id, track := range r.tracks {
			if sent[id] || p.owned[id] {
				continue
			}
			if _, err := p.pc.AddTrack(track); err != nil {
				return false
			}
			changed = true
		}

		if !changed && p.pc.LocalDescription() != nil {
			continue
		}
		if p.pc.SignalingState() != webrtc.SignalingStateStable {
			return false
		}

		offer, err := p.pc.CreateOffer(nil)
		if err != nil {
			return false
		}
		if err := p.pc.SetLocalDescription(offer); err != nil {
			return false
		}
		s.signaler.SendOffer(p.userID, offer)
	}
	return true
}
