package voice

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

func watchConnection(pc *webrtc.PeerConnection, userID uuid.UUID, carries string, lost func()) {
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateConnected:
			local, remote := selectedPair(pc)
			slog.Info("voice: media path established",
				"user_id", userID, "carries", carries,
				"ours", local, "theirs", remote)

		case webrtc.PeerConnectionStateFailed:
			slog.Warn("voice: no path for media, giving up on this peer",
				"user_id", userID, "carries", carries,
				"means", "every candidate pair failed its connectivity check",
				"fix", "open the WEBRTC_UDP_PORT_MIN-MAX range, set WEBRTC_PUBLIC_IP if "+
					"this host sits behind a NAT, or configure TURN for peers that can "+
					"reach neither")
			if lost != nil {
				lost()
			}

		case webrtc.PeerConnectionStateDisconnected:
			slog.Info("voice: media path lost, waiting for it to come back",
				"user_id", userID, "carries", carries)

		case webrtc.PeerConnectionStateClosed:
			if lost != nil {
				lost()
			}
		}
	})
}

func selectedPair(pc *webrtc.PeerConnection) (string, string) {
	sctp := pc.SCTP()
	if sctp == nil {
		return "", ""
	}
	dtls := sctp.Transport()
	if dtls == nil {
		return "", ""
	}
	transport := dtls.ICETransport()
	if transport == nil {
		return "", ""
	}
	pair, err := transport.GetSelectedCandidatePair()
	if err != nil || pair == nil {
		return "", ""
	}
	return describe(pair.Local), describe(pair.Remote)
}

func describe(c *webrtc.ICECandidate) string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s %s:%d/%s", c.Typ, c.Address, c.Port, c.Protocol)
}
