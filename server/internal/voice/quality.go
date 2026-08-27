package voice

import (
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

const (
	qualityInterval = 5 * time.Second

	fairLoss = 0.02
	poorLoss = 0.08
	fairRTT  = 200 * time.Millisecond
	poorRTT  = 400 * time.Millisecond

	QualityGood = "good"
	QualityFair = "fair"
	QualityPoor = "poor"
)

type Quality struct {
	UserID uuid.UUID
	Grade  string
	Loss   float64
	RTT    time.Duration
}

func grade(loss float64, rtt time.Duration) string {
	switch {
	case loss >= poorLoss || rtt >= poorRTT:
		return QualityPoor
	case loss >= fairLoss || rtt >= fairRTT:
		return QualityFair
	default:
		return QualityGood
	}
}

func measure(pc *webrtc.PeerConnection) (Quality, bool) {
	var (
		loss  float64
		rtt   time.Duration
		heard bool
	)
	for _, stat := range pc.GetStats() {
		remote, ok := stat.(webrtc.RemoteInboundRTPStreamStats)
		if !ok {
			continue
		}
		heard = true
		if remote.FractionLost > loss {
			loss = remote.FractionLost
		}
		if seen := time.Duration(remote.RoundTripTime * float64(time.Second)); seen > rtt {
			rtt = seen
		}
	}
	if !heard {
		return Quality{}, false
	}
	return Quality{Grade: grade(loss, rtt), Loss: loss, RTT: rtt}, true
}

func (s *SFU) sampleQuality() {
	ticker := time.NewTicker(qualityInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			for channelID, changes := range s.qualityChanges() {
				for _, q := range changes {
					s.signaler.QualityChanged(channelID, q)
				}
			}
		}
	}
}

func (s *SFU) qualityChanges() map[uuid.UUID][]Quality {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[uuid.UUID][]Quality)
	for channelID, r := range s.rooms {
		for userID, p := range r.peers {
			if p.pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
				continue
			}
			q, ok := measure(p.pc)
			if !ok || q.Grade == p.graded {
				continue
			}
			p.graded = q.Grade
			q.UserID = userID
			out[channelID] = append(out[channelID], q)
		}
	}
	return out
}
