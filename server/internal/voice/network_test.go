package voice

import (
	"strings"
	"testing"
)

func TestNetworkAcceptsAPortRangeAndPublicIP(t *testing.T) {
	_, err := New(newRecordingSignaler(), nil, Network{
		PublicIP:   "203.0.113.10",
		UDPPortMin: 50000,
		UDPPortMax: 50999,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNetworkZeroValueKeepsTheOldBehaviour(t *testing.T) {
	if _, err := New(newRecordingSignaler(), nil, Network{}); err != nil {
		t.Fatalf("an unset Network must still build an SFU: %v", err)
	}
}

func TestNetworkRefusesABrokenPortRange(t *testing.T) {
	cases := map[string]Network{
		"inverted":    {UDPPortMin: 51000, UDPPortMax: 50000},
		"missing end": {UDPPortMin: 50000},
		"off the top": {UDPPortMin: 50000, UDPPortMax: 70000},
	}

	for name, network := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(newRecordingSignaler(), nil, network)
			if err == nil {
				t.Fatal("New accepted the range; a silently ignored range means the firewall holes are wrong")
			}
			if !strings.Contains(err.Error(), "udp port range") {
				t.Errorf("err = %v, want it to name the udp port range so the operator knows which variable to fix", err)
			}
		})
	}
}
