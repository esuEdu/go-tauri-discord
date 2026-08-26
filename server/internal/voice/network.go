package voice

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/pion/webrtc/v4"
)

type Network struct {
	PublicIP   string
	UDPPortMin int
	UDPPortMax int
}

func NoPublicAddress() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		prefix, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip, ok := netip.AddrFromSlice(prefix.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() || ip.IsUnspecified() {
			continue
		}
		return false
	}
	return true
}

func (n Network) settingEngine() (webrtc.SettingEngine, error) {
	var se webrtc.SettingEngine

	if n.UDPPortMin != 0 || n.UDPPortMax != 0 {
		if n.UDPPortMin <= 0 || n.UDPPortMax <= 0 {
			return se, fmt.Errorf("voice: udp port range needs both ends, got %d-%d", n.UDPPortMin, n.UDPPortMax)
		}
		if n.UDPPortMin > 65535 || n.UDPPortMax > 65535 {
			return se, fmt.Errorf("voice: udp port range %d-%d leaves the port space", n.UDPPortMin, n.UDPPortMax)
		}
		if n.UDPPortMin > n.UDPPortMax {
			return se, fmt.Errorf("voice: udp port range %d-%d is inverted", n.UDPPortMin, n.UDPPortMax)
		}
		if err := se.SetEphemeralUDPPortRange(uint16(n.UDPPortMin), uint16(n.UDPPortMax)); err != nil {
			return se, fmt.Errorf("voice: udp port range: %w", err)
		}
	}

	if n.PublicIP != "" {
		if err := se.SetICEAddressRewriteRules(webrtc.ICEAddressRewriteRule{
			External:        []string{n.PublicIP},
			AsCandidateType: webrtc.ICECandidateTypeHost,
			Mode:            webrtc.ICEAddressRewriteReplace,
		}); err != nil {
			return se, fmt.Errorf("voice: public ip %q: %w", n.PublicIP, err)
		}
	}

	return se, nil
}
