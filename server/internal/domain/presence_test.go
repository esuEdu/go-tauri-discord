package domain

import "testing"

func TestResolveTellsOthersOnlyWhatTheyMayKnow(t *testing.T) {
	cases := []struct {
		name      string
		chosen    Status
		connected bool
		idle      bool
		want      Status
		why       string
	}{
		{"invisible while connected", StatusInvisible, true, false, StatusOffline,
			"invisible resolved to something other than offline, so the whole point of it is gone"},
		{"invisible and idle", StatusInvisible, true, true, StatusOffline,
			"idleness leaked that an invisible person is connected"},
		{"invisible and gone", StatusInvisible, false, false, StatusOffline, ""},
		{"online and connected", StatusOnline, true, false, StatusOnline, ""},
		{"online and idle", StatusOnline, true, true, StatusAway,
			"automatic idleness must read as away; there is no fifth value on the wire"},
		{"busy and idle", StatusBusy, true, true, StatusBusy,
			"idleness overrode a chosen status; a choice is a statement and outranks a timer"},
		{"away and idle", StatusAway, true, true, StatusAway, ""},
		{"busy and gone", StatusBusy, false, false, StatusOffline,
			"a chosen status survived the app closing"},
		{"away and gone", StatusAway, false, false, StatusOffline, ""},
	}

	for _, c := range cases {
		got := Resolve(c.chosen, c.connected, c.idle)
		if got != c.want {
			why := c.why
			if why == "" {
				why = "unexpected resolution"
			}
			t.Errorf("%s: Resolve(%q, connected=%v, idle=%v) = %q, want %q: %s",
				c.name, c.chosen, c.connected, c.idle, got, c.want, why)
		}
	}
}

func TestNothingResolvesToInvisible(t *testing.T) {
	for _, chosen := range []Status{StatusOnline, StatusAway, StatusBusy, StatusInvisible} {
		for _, connected := range []bool{true, false} {
			for _, idle := range []bool{true, false} {
				if got := Resolve(chosen, connected, idle); got == StatusInvisible {
					t.Fatalf("Resolve(%q, %v, %v) = invisible; invisible has no place on the wire, "+
						"and a payload that can carry it is a payload that can leak it",
						chosen, connected, idle)
				}
			}
		}
	}
}

func TestOfflineCannotBeChosen(t *testing.T) {
	if _, err := ChosenStatus("offline"); err == nil {
		t.Error("offline was accepted as a choice, so somebody can claim to be gone while connected " +
			"by a route that is not invisible and does not behave like it")
	}
	for _, ok := range []string{"online", "away", "busy", "invisible"} {
		if _, err := ChosenStatus(ok); err != nil {
			t.Errorf("ChosenStatus(%q) was refused: %v", ok, err)
		}
	}
}
