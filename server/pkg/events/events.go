package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Opcode int

const (
	OpDispatch       Opcode = 0
	OpHeartbeat      Opcode = 1
	OpIdentify       Opcode = 2
	OpHello          Opcode = 3
	OpHeartbeatAck   Opcode = 4
	OpResume         Opcode = 5
	OpInvalidSession Opcode = 6
	OpVoiceState     Opcode = 7
	OpVoiceOffer     Opcode = 8
	OpVoiceAnswer    Opcode = 9
	OpVoiceCandidate Opcode = 10
	OpVoiceResync    Opcode = 11
	OpVoiceScreen    Opcode = 12
	OpVoiceMute      Opcode = 13
)

type EventType string

const (
	EventReady             EventType = "READY"
	EventGuildCreate       EventType = "GUILD_CREATE"
	EventChannelCreate     EventType = "CHANNEL_CREATE"
	EventMessageCreate     EventType = "MESSAGE_CREATE"
	EventMessageUpdate     EventType = "MESSAGE_UPDATE"
	EventMessageDelete     EventType = "MESSAGE_DELETE"
	EventTypingStart       EventType = "TYPING_START"
	EventPresenceUpdate    EventType = "PRESENCE_UPDATE"
	EventVoiceStateUpdate  EventType = "VOICE_STATE_UPDATE"
	EventVoiceScreenUpdate EventType = "VOICE_SCREEN_UPDATE"
)

type Frame struct {
	Op Opcode          `json:"op"`
	T  EventType       `json:"t,omitempty"`
	S  int64           `json:"s,omitempty"`
	D  json.RawMessage `json:"d,omitempty"`
}

type Hello struct {
	HeartbeatIntervalMS int `json:"heartbeat_interval_ms"`
}

type Identify struct {
	Token string `json:"token"`
}

type Resume struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int64  `json:"seq"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	AvatarKey *string   `json:"avatar_key"`
}

type Guild struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	OwnerID uuid.UUID `json:"owner_id"`
	IconKey *string   `json:"icon_key"`
}

type Channel struct {
	ID            uuid.UUID  `json:"id"`
	GuildID       uuid.UUID  `json:"guild_id"`
	ParentID      *uuid.UUID `json:"parent_id"`
	Kind          string     `json:"kind"`
	Name          string     `json:"name"`
	Topic         *string    `json:"topic"`
	Position      int32      `json:"position"`
	LastMessageID *uuid.UUID `json:"last_message_id"`
}

type Member struct {
	GuildID uuid.UUID `json:"guild_id"`
	User    User      `json:"user"`
}

type ReadState struct {
	ChannelID         uuid.UUID  `json:"channel_id"`
	LastReadMessageID *uuid.UUID `json:"last_read_message_id"`
}

type Role struct {
	ID          uuid.UUID `json:"id"`
	GuildID     uuid.UUID `json:"guild_id"`
	Name        string    `json:"name"`
	Permissions int64     `json:"permissions"`
	Position    int32     `json:"position"`
	IsDefault   bool      `json:"is_default"`
}

type Overwrite struct {
	ChannelID  uuid.UUID `json:"channel_id"`
	TargetID   uuid.UUID `json:"target_id"`
	TargetType string    `json:"target_type"`
	Allow      int64     `json:"allow"`
	Deny       int64     `json:"deny"`
}

type Attachment struct {
	ID          uuid.UUID `json:"id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	URL         string    `json:"url"`
}

type Message struct {
	ID          uuid.UUID    `json:"id"`
	ChannelID   uuid.UUID    `json:"channel_id"`
	Author      User         `json:"author"`
	Content     string       `json:"content"`
	CreatedAt   time.Time    `json:"created_at"`
	EditedAt    *time.Time   `json:"edited_at"`
	Attachments []Attachment `json:"attachments"`
}

type Ready struct {
	SessionID  string      `json:"session_id"`
	User       User        `json:"user"`
	Guilds     []Guild     `json:"guilds"`
	Channels   []Channel   `json:"channels"`
	Members    []Member    `json:"members"`
	ReadStates []ReadState `json:"read_states"`
	Online     []uuid.UUID `json:"online"`
}

type MessageDelete struct {
	ID        uuid.UUID `json:"id"`
	ChannelID uuid.UUID `json:"channel_id"`
}

type TypingStart struct {
	ChannelID uuid.UUID `json:"channel_id"`
	UserID    uuid.UUID `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}

type PresenceUpdate struct {
	UserID uuid.UUID `json:"user_id"`
	Status string    `json:"status"`
}

type VoiceStateRequest struct {
	ChannelID *uuid.UUID `json:"channel_id"`
	SelfMute  bool       `json:"self_mute"`
	SelfDeaf  bool       `json:"self_deaf"`
}

type SessionDescription struct {
	Type           string  `json:"type"`
	SDP            string  `json:"sdp"`
	ScreenMid      *string `json:"screen_mid"`
	ScreenAudioMid *string `json:"screen_audio_mid"`
}

type ICECandidate struct {
	Candidate        string  `json:"candidate"`
	SDPMid           *string `json:"sdp_mid"`
	SDPMLineIndex    *uint16 `json:"sdp_mline_index"`
	UsernameFragment *string `json:"username_fragment"`
}

type VoiceStateUpdate struct {
	GuildID   uuid.UUID  `json:"guild_id"`
	ChannelID *uuid.UUID `json:"channel_id"`
	UserID    uuid.UUID  `json:"user_id"`
	SelfMute  bool       `json:"self_mute"`
	SelfDeaf  bool       `json:"self_deaf"`
}

type VoiceMuteRequest struct {
	SelfMute bool `json:"self_mute"`
}

type VoiceScreenRequest struct {
	Active bool `json:"active"`
}

type VoiceScreenUpdate struct {
	GuildID   uuid.UUID `json:"guild_id"`
	ChannelID uuid.UUID `json:"channel_id"`
	UserID    uuid.UUID `json:"user_id"`
	StreamID  string    `json:"stream_id"`
	Active    bool      `json:"active"`
}

func NewDispatch(t EventType, d any) (Frame, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return Frame{}, err
	}
	return Frame{Op: OpDispatch, T: t, D: raw}, nil
}
