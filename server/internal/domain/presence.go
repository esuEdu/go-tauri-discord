package domain

type Status string

const (
	StatusOnline    Status = "online"
	StatusAway      Status = "away"
	StatusBusy      Status = "busy"
	StatusInvisible Status = "invisible"
	StatusOffline   Status = "offline"
)

func ChosenStatus(raw string) (Status, error) {
	switch Status(raw) {
	case StatusOnline, StatusAway, StatusBusy, StatusInvisible:
		return Status(raw), nil
	case StatusOffline:
		return "", Invalid("offline is what happens when you close the app, not something to choose")
	default:
		return "", Invalid("status must be one of online, away, busy or invisible")
	}
}

func Resolve(chosen Status, connected, idle bool) Status {
	if !connected || chosen == StatusInvisible {
		return StatusOffline
	}
	if idle && chosen == StatusOnline {
		return StatusAway
	}
	return chosen
}
