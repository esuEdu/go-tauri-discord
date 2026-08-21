package voice

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/webrtc/v4"
)

const (
	initialEstimate  = 1_000_000
	sampleInterval   = 5 * time.Second
	significantDelta = 1.5
)

type Estimate struct {
	UserID uuid.UUID
	Bits   int
}

func (s *SFU) newPeerConnection() (*webrtc.PeerConnection, cc.BandwidthEstimator, error) {
	s.birth.Lock()
	defer s.birth.Unlock()

	s.arriving = nil
	pc, err := s.api.NewPeerConnection(s.config)
	estimator := s.arriving
	s.arriving = nil

	if err != nil {
		return nil, nil, err
	}
	return pc, estimator, nil
}

func (s *SFU) Estimates(channelID uuid.UUID) []Estimate {
	s.mu.Lock()
	r := s.rooms[channelID]
	if r == nil {
		s.mu.Unlock()
		return nil
	}
	peers := make([]*peer, 0, len(r.peers))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	s.mu.Unlock()

	out := make([]Estimate, 0, len(peers))
	for _, p := range peers {
		if p.estimate == nil {
			continue
		}
		out = append(out, Estimate{UserID: p.userID, Bits: p.estimate.GetTargetBitrate()})
	}
	return out
}

func (s *SFU) sampleBandwidth() {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			for _, channelID := range s.sharingChannels() {
				s.viewerBandwidth(channelID).log(channelID)
			}
		}
	}
}

func (s *SFU) sharingChannels() []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]uuid.UUID, 0, len(s.rooms))
	for channelID, r := range s.rooms {
		if len(r.screens) > 0 {
			out = append(out, channelID)
		}
	}
	return out
}

type shareBandwidth struct {
	Viewers int
	Lowest  int
	Highest int
}

func (b shareBandwidth) spread() float64 {
	if b.Lowest <= 0 {
		return 0
	}
	return float64(b.Highest) / float64(b.Lowest)
}

func (b shareBandwidth) log(channelID uuid.UUID) {
	if b.Viewers == 0 {
		return
	}
	slog.Info("voice: screen bandwidth",
		"channel_id", channelID,
		"viewers", b.Viewers,
		"lowest_bps", b.Lowest,
		"highest_bps", b.Highest,
		"spread", b.spread(),
		"one_stream_fits_all", b.spread() < significantDelta)
}

func (s *SFU) viewerBandwidth(channelID uuid.UUID) shareBandwidth {
	s.mu.Lock()
	r := s.rooms[channelID]
	if r == nil {
		s.mu.Unlock()
		return shareBandwidth{}
	}
	sharing := make(map[uuid.UUID]bool, len(r.screens))
	for userID := range r.screens {
		sharing[userID] = true
	}
	s.mu.Unlock()

	var summary shareBandwidth
	for _, e := range s.Estimates(channelID) {
		if sharing[e.UserID] {
			continue
		}
		summary.Viewers++
		if summary.Lowest == 0 || e.Bits < summary.Lowest {
			summary.Lowest = e.Bits
		}
		if e.Bits > summary.Highest {
			summary.Highest = e.Bits
		}
		slog.Debug("voice: viewer bandwidth",
			"channel_id", channelID, "user_id", e.UserID, "bits_per_second", e.Bits)
	}
	return summary
}
