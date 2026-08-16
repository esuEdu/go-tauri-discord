package domain

const (
	ChannelText     = "text"
	ChannelVoice    = "voice"
	ChannelCategory = "category"
)

func ValidChannelKind(kind string) bool {
	switch kind {
	case ChannelText, ChannelVoice, ChannelCategory:
		return true
	default:
		return false
	}
}
