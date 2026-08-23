package voice

import (
	"errors"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

var ErrNotConnected = errors.New("voice: peer is not connected")

var ErrNotAllowed = errors.New("voice: not allowed to share a screen here")

type Signaler interface {
	SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription)
	SendCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit)
	VoiceClosed(userID uuid.UUID)
	ScreenChanged(channelID, userID uuid.UUID, streamID string, active bool)
}

type SFU struct {
	api      *webrtc.API
	config   webrtc.Configuration
	signaler Signaler

	mu    sync.Mutex
	rooms map[uuid.UUID]*room
	homes map[uuid.UUID]uuid.UUID

	birth    sync.Mutex
	arriving cc.BandwidthEstimator

	publishMu       sync.Mutex
	publishers      map[uuid.UUID]*publisher
	early           map[uuid.UUID][]webrtc.ICECandidateInit
	publishSignaler PublishSignaler
	stop            chan struct{}
	stopOnce        sync.Once
}

type room struct {
	channelID uuid.UUID
	peers     map[uuid.UUID]*peer
	tracks    map[string]*webrtc.TrackLocalStaticRTP
	screens   map[uuid.UUID]string
	keyframes map[string]func()
	layers    map[string]layer
}

type layer struct {
	owner uuid.UUID
	rid   string
}

type peer struct {
	userID           uuid.UUID
	pc               *webrtc.PeerConnection
	owned            map[string]bool
	mayStream        bool
	screenTrack      *webrtc.TrackLocalStaticRTP
	screenAudioTrack *webrtc.TrackLocalStaticRTP
	screenAsk        func()
	estimate         cc.BandwidthEstimator
	muted            bool
	ignored          map[uuid.UUID]bool
	sizes            map[uuid.UUID]string
	redo             bool

	mu         sync.Mutex
	pending    []webrtc.ICECandidateInit
	haveRemote bool
}

const (
	signalAttempts   = 25
	signalBackoff    = 2 * time.Second
	rtpBufferSize    = 1500
	keyframeCooldown = 500 * time.Millisecond
)

type keyframeRequester struct {
	pc   *webrtc.PeerConnection
	ssrc webrtc.SSRC

	mu   sync.Mutex
	last time.Time
}

func (k *keyframeRequester) ask() {
	k.mu.Lock()
	if !k.last.IsZero() && time.Since(k.last) < keyframeCooldown {
		k.mu.Unlock()
		return
	}
	k.last = time.Now()
	k.mu.Unlock()

	_ = k.pc.WriteRTCP([]rtcp.Packet{
		&rtcp.PictureLossIndication{MediaSSRC: uint32(k.ssrc)},
	})
}

func relayKeyframeRequests(sender *webrtc.RTPSender, ask func()) {
	if sender == nil {
		return
	}
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			return
		}
		for _, packet := range packets {
			switch packet.(type) {
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				ask()
			}
		}
	}
}

func New(signaler Signaler, iceServers []string) (*SFU, error) {
	servers := make([]webrtc.ICEServer, 0, len(iceServers))
	for _, url := range iceServers {
		if url != "" {
			servers = append(servers, webrtc.ICEServer{URLs: []string{url}})
		}
	}

	s := &SFU{
		config:     webrtc.Configuration{ICEServers: servers},
		signaler:   signaler,
		rooms:      make(map[uuid.UUID]*room),
		homes:      make(map[uuid.UUID]uuid.UUID),
		publishers: make(map[uuid.UUID]*publisher),
		early:      make(map[uuid.UUID][]webrtc.ICECandidateInit),
		stop:       make(chan struct{}),
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	registry := &interceptor.Registry{}

	congestion, err := cc.NewInterceptor(newEstimator)
	if err != nil {
		return nil, err
	}
	congestion.OnNewPeerConnection(func(_ string, estimator cc.BandwidthEstimator) {
		s.arriving = estimator
	})
	registry.Add(congestion)

	if err := webrtc.ConfigureTWCCHeaderExtensionSender(mediaEngine, registry); err != nil {
		return nil, err
	}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, registry); err != nil {
		return nil, err
	}

	s.api = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(registry),
	)

	go s.sampleBandwidth()
	return s, nil
}

func (s *SFU) Join(channelID, userID uuid.UUID, mayStream bool) error {
	s.Leave(userID)

	pc, estimator, err := s.newPeerConnection()
	if err != nil {
		return err
	}
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		pc.Close()
		return err
	}

	p := &peer{
		userID: userID, pc: pc, owned: make(map[string]bool),
		estimate: estimator, ignored: make(map[uuid.UUID]bool),
		sizes: make(map[uuid.UUID]string), mayStream: mayStream,
	}

	s.mu.Lock()
	r, ok := s.rooms[channelID]
	if !ok {
		r = &room{
			channelID: channelID,
			peers:     make(map[uuid.UUID]*peer),
			tracks:    make(map[string]*webrtc.TrackLocalStaticRTP),
			screens:   make(map[uuid.UUID]string),
			keyframes: make(map[string]func()),
			layers:    make(map[string]layer),
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
			s.leave(userID, p)
		}
	})

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		s.forward(r, p, remote, SourceMicrophone)
	})

	s.mu.Lock()
	s.signalLocked(r)
	s.mu.Unlock()
	return nil
}

func (s *SFU) forward(r *room, p *peer, remote *webrtc.TrackRemote, source Source) {
	s.forwardLayer(r, p, p.pc, remote, source, remote.RID())
}

func (s *SFU) forwardLayer(r *room, p *peer, from *webrtc.PeerConnection, remote *webrtc.TrackRemote, source Source, rid string) {
	trackID := TrackName(source, p.userID, uint32(remote.SSRC()))

	local, err := webrtc.NewTrackLocalStaticRTP(
		remote.Codec().RTPCodecCapability, trackID, trackID)
	if err != nil {
		slog.Error("voice: create local track", "error", err)
		return
	}

	s.mu.Lock()
	r.tracks[local.ID()] = local
	p.owned[local.ID()] = true
	switch source {
	case SourceScreen:
		r.screens[p.userID] = local.StreamID()
		r.keyframes[local.ID()] = (&keyframeRequester{pc: from, ssrc: remote.SSRC()}).ask
		r.layers[local.ID()] = layer{owner: p.userID, rid: rid}
		p.screenTrack = local
		p.screenAsk = r.keyframes[local.ID()]
	case SourceScreenAudio:
		p.screenAudioTrack = local
	}
	s.signalLocked(r)
	s.mu.Unlock()

	if source == SourceScreen {
		s.signaler.ScreenChanged(r.channelID, p.userID, local.StreamID(), true)
	}

	defer func() {
		s.mu.Lock()
		_, wasLive := r.tracks[local.ID()]
		delete(r.tracks, local.ID())
		delete(p.owned, local.ID())
		delete(r.keyframes, local.ID())
		switch source {
		case SourceScreen:
			delete(r.layers, local.ID())
			if r.screens[p.userID] == local.StreamID() {
				delete(r.screens, p.userID)
			}
			if p.screenTrack == local {
				p.screenTrack = nil
				p.screenAsk = nil
			}
		case SourceScreenAudio:
			if p.screenAudioTrack == local {
				p.screenAudioTrack = nil
			}
		}
		s.signalLocked(r)
		s.mu.Unlock()

		if source == SourceScreen && wasLive {
			s.signaler.ScreenChanged(r.channelID, p.userID, local.StreamID(), false)
		}
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
	s.leave(userID, nil)
}

func (s *SFU) leave(userID uuid.UUID, only *peer) {
	s.mu.Lock()
	channelID, ok := s.homes[userID]
	if !ok {
		s.mu.Unlock()
		return
	}

	r := s.rooms[channelID]
	if r == nil {
		delete(s.homes, userID)
		s.mu.Unlock()
		return
	}
	p := r.peers[userID]
	if only != nil && p != only {
		s.mu.Unlock()
		return
	}
	delete(s.homes, userID)
	delete(r.peers, userID)
	delete(r.screens, userID)

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
	s.StopPublishing(userID)
}

func (s *SFU) Answer(userID uuid.UUID, sdp webrtc.SessionDescription) error {
	p := s.peerFor(userID)
	if p == nil {
		return ErrNotConnected
	}
	if err := p.pc.SetRemoteDescription(sdp); err != nil {
		return err
	}
	if err := p.drainCandidates(); err != nil {
		return err
	}
	s.refreshSubscribedScreens(userID)
	return nil
}

func (s *SFU) refreshSubscribedScreens(userID uuid.UUID) {
	s.mu.Lock()
	channelID, ok := s.homes[userID]
	if !ok {
		s.mu.Unlock()
		return
	}
	r := s.rooms[channelID]
	if r == nil {
		s.mu.Unlock()
		return
	}
	p := r.peers[userID]
	if p == nil {
		s.mu.Unlock()
		return
	}

	asks := make([]func(), 0, len(r.keyframes))
	for id, ask := range r.keyframes {
		if !p.owned[id] {
			asks = append(asks, ask)
		}
	}
	s.mu.Unlock()

	for _, ask := range asks {
		go ask()
	}
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

func (s *SFU) Resync(userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	channelID, ok := s.homes[userID]
	if !ok {
		return ErrNotConnected
	}
	r := s.rooms[channelID]
	if r == nil {
		return ErrNotConnected
	}
	p := r.peers[userID]
	if p == nil {
		return ErrNotConnected
	}

	p.redo = true
	s.signalLocked(r)
	return nil
}

func (s *SFU) SetScreenActive(userID uuid.UUID, active bool) error {
	s.mu.Lock()

	channelID, ok := s.homes[userID]
	if !ok {
		s.mu.Unlock()
		return ErrNotConnected
	}
	r := s.rooms[channelID]
	if r == nil {
		s.mu.Unlock()
		return ErrNotConnected
	}
	p := r.peers[userID]
	if p == nil {
		s.mu.Unlock()
		return ErrNotConnected
	}

	video := p.screenTrack
	changed := false
	for _, track := range p.sharedTracks() {
		if _, live := r.tracks[track.ID()]; live == active {
			continue
		}
		if active {
			r.tracks[track.ID()] = track
			if track == video {
				r.keyframes[track.ID()] = p.screenAsk
				r.screens[userID] = track.StreamID()
			}
		} else {
			delete(r.tracks, track.ID())
			if track == video {
				delete(r.keyframes, track.ID())
				delete(r.screens, userID)
			}
		}
		changed = changed || track == video
	}

	p.redo = true
	s.signalLocked(r)
	s.mu.Unlock()

	if changed {
		s.signaler.ScreenChanged(channelID, userID, video.StreamID(), active)
	}
	return nil
}

func (p *peer) wants(r *room, trackID string) bool {
	source, owner, ok := ParseTrackName(trackID)
	if !ok || source != SourceScreen {
		return true
	}
	if p.ignored[owner] {
		return false
	}

	known, layered := r.layers[trackID]
	if !layered || known.rid == "" {
		return true
	}
	return known.rid == p.sizeFor(owner)
}

func (p *peer) sizeFor(owner uuid.UUID) string {
	if chosen := p.sizes[owner]; chosen != "" {
		return chosen
	}
	return DefaultLayer
}

func (s *SFU) SetWatching(viewerID, sharerID uuid.UUID, watching bool, size string) error {
	s.mu.Lock()

	channelID, ok := s.homes[viewerID]
	if !ok {
		s.mu.Unlock()
		return ErrNotConnected
	}
	r := s.rooms[channelID]
	if r == nil {
		s.mu.Unlock()
		return ErrNotConnected
	}
	p := r.peers[viewerID]
	if p == nil {
		s.mu.Unlock()
		return ErrNotConnected
	}

	if watching {
		delete(p.ignored, sharerID)
		if KnownLayer(size) {
			p.sizes[sharerID] = size
		}
	} else {
		p.ignored[sharerID] = true
	}

	asks := make([]func(), 0, 1)
	if watching {
		wanted := p.sizeFor(sharerID)
		for id, ask := range r.keyframes {
			known, layered := r.layers[id]
			if !layered || known.owner != sharerID {
				continue
			}
			if known.rid == "" || known.rid == wanted {
				asks = append(asks, ask)
			}
		}
	}

	p.redo = true
	s.signalLocked(r)
	s.mu.Unlock()

	for _, ask := range asks {
		go ask()
	}
	return nil
}

func (p *peer) sharedTracks() []*webrtc.TrackLocalStaticRTP {
	out := make([]*webrtc.TrackLocalStaticRTP, 0, 2)
	if p.screenTrack != nil {
		out = append(out, p.screenTrack)
	}
	if p.screenAudioTrack != nil {
		out = append(out, p.screenAudioTrack)
	}
	return out
}

func (s *SFU) Sharers(channelID uuid.UUID) map[uuid.UUID]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := s.rooms[channelID]
	if r == nil {
		return nil
	}
	out := make(map[uuid.UUID]string, len(r.screens))
	maps.Copy(out, r.screens)
	return out
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

func (s *SFU) Muted(channelID uuid.UUID) map[uuid.UUID]bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := s.rooms[channelID]
	if r == nil {
		return nil
	}
	out := make(map[uuid.UUID]bool, len(r.peers))
	for id, p := range r.peers {
		if p.muted {
			out[id] = true
		}
	}
	return out
}

func (s *SFU) SetMuted(userID uuid.UUID, muted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	channelID, ok := s.homes[userID]
	if !ok {
		return ErrNotConnected
	}
	r := s.rooms[channelID]
	if r == nil {
		return ErrNotConnected
	}
	p := r.peers[userID]
	if p == nil {
		return ErrNotConnected
	}

	p.muted = muted
	return nil
}

func (s *SFU) Close() {
	s.stopOnce.Do(func() { close(s.stop) })

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

	s.publishMu.Lock()
	publishers := make([]*publisher, 0, len(s.publishers))
	for _, p := range s.publishers {
		publishers = append(publishers, p)
	}
	s.publishers = make(map[uuid.UUID]*publisher)
	s.early = make(map[uuid.UUID][]webrtc.ICECandidateInit)
	s.publishMu.Unlock()

	for _, p := range publishers {
		p.pc.Close()
	}
}

func (s *SFU) roomFor(userID uuid.UUID) (*room, *peer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	channelID, ok := s.homes[userID]
	if !ok {
		return nil, nil
	}
	r := s.rooms[channelID]
	if r == nil {
		return nil, nil
	}
	return r, r.peers[userID]
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

		changed := false
		sent := map[string]bool{}
		for _, sender := range p.pc.GetSenders() {
			track := sender.Track()
			if track == nil {
				continue
			}
			_, live := r.tracks[track.ID()]
			if !live || !p.wants(r, track.ID()) {
				if err := p.pc.RemoveTrack(sender); err != nil {
					return false
				}
				changed = true
				continue
			}
			sent[track.ID()] = true
		}

		for id, track := range r.tracks {
			if sent[id] || p.owned[id] || !p.wants(r, id) {
				continue
			}
			transceiver, err := p.pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionSendonly,
			})
			if err != nil {
				return false
			}
			if ask := r.keyframes[id]; ask != nil {
				go relayKeyframeRequests(transceiver.Sender(), ask)
			}
			changed = true
		}

		if !changed && !p.redo && p.pc.LocalDescription() != nil {
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
		p.redo = false
		s.signaler.SendOffer(p.userID, offer)
	}
	return true
}
