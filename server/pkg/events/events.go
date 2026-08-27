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
	OpVoiceWatch     Opcode = 14
	OpScreenPublish  Opcode = 15
	OpScreenAnswer   Opcode = 16
	OpScreenIce      Opcode = 17
)

type EventType string

const (
	EventReady             EventType = "READY"
	EventGuildCreate       EventType = "GUILD_CREATE"
	EventChannelCreate     EventType = "CHANNEL_CREATE"
	EventChannelUpdate     EventType = "CHANNEL_UPDATE"
	EventMessageCreate     EventType = "MESSAGE_CREATE"
	EventMessageUpdate     EventType = "MESSAGE_UPDATE"
	EventMessageDelete     EventType = "MESSAGE_DELETE"
	EventReactionAdd       EventType = "MESSAGE_REACTION_ADD"
	EventReactionRemove    EventType = "MESSAGE_REACTION_REMOVE"
	EventTypingStart       EventType = "TYPING_START"
	EventPresenceUpdate    EventType = "PRESENCE_UPDATE"
	EventPermissionsUpdate EventType = "PERMISSIONS_UPDATE"
	EventGuildMemberAdd    EventType = "GUILD_MEMBER_ADD"
	EventGuildMemberRemove EventType = "GUILD_MEMBER_REMOVE"
	EventGuildRemove       EventType = "GUILD_REMOVE"
	EventVoiceStateUpdate  EventType = "VOICE_STATE_UPDATE"
	EventVoiceScreenUpdate EventType = "VOICE_SCREEN_UPDATE"
	EventVoiceQuality      EventType = "VOICE_QUALITY"
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
	ID            uuid.UUID `json:"id"`
	Username      string    `json:"username"`
	Discriminator string    `json:"discriminator"`
	AvatarKey     *string   `json:"avatar_key"`
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

type GuildRemoval struct {
	GuildID uuid.UUID `json:"guild_id"`
	UserID  uuid.UUID `json:"user_id"`
	Banned  bool      `json:"banned"`
}

type ChannelPermission struct {
	ChannelID   uuid.UUID `json:"channel_id"`
	Permissions int64     `json:"permissions"`
}

type GuildPermissions struct {
	GuildID     uuid.UUID           `json:"guild_id"`
	Permissions int64               `json:"permissions"`
	Channels    []ChannelPermission `json:"channels"`
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

type Reaction struct {
	Emoji string `json:"emoji"`
	Count int32  `json:"count"`
	Mine  bool   `json:"mine"`
}

type ReplyPreview struct {
	MessageID      uuid.UUID `json:"message_id"`
	Author         *User     `json:"author"`
	Content        string    `json:"content"`
	Truncated      bool      `json:"truncated"`
	HasAttachments bool      `json:"has_attachments"`
	Deleted        bool      `json:"deleted"`
}

type Message struct {
	ID          uuid.UUID     `json:"id"`
	ChannelID   uuid.UUID     `json:"channel_id"`
	Author      User          `json:"author"`
	Content     string        `json:"content"`
	CreatedAt   time.Time     `json:"created_at"`
	EditedAt    *time.Time    `json:"edited_at"`
	Attachments []Attachment  `json:"attachments"`
	Reactions   []Reaction    `json:"reactions"`
	ReplyTo     *ReplyPreview `json:"reply_to"`
}

type Ready struct {
	SessionID  string             `json:"session_id"`
	User       User               `json:"user"`
	Guilds     []Guild            `json:"guilds"`
	Channels   []Channel          `json:"channels"`
	Members    []Member           `json:"members"`
	ReadStates []ReadState        `json:"read_states"`
	Allowed    []GuildPermissions `json:"allowed"`
	Online     []uuid.UUID        `json:"online"`
	ICEServers []ICEServer        `json:"ice_servers"`
	Voice      []VoiceStateUpdate `json:"voice"`
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type MessageDelete struct {
	ID        uuid.UUID `json:"id"`
	ChannelID uuid.UUID `json:"channel_id"`
}

type MessageReaction struct {
	MessageID uuid.UUID `json:"message_id"`
	ChannelID uuid.UUID `json:"channel_id"`
	UserID    uuid.UUID `json:"user_id"`
	Emoji     string    `json:"emoji"`
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
	Type string `json:"type"`
	SDP  string `json:"sdp"`
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

type ScreenPublish struct {
	SDP string `json:"sdp"`
}

type VoiceWatchRequest struct {
	UserID   uuid.UUID `json:"user_id"`
	Watching bool      `json:"watching"`
	Size     string    `json:"size"`
}

type VoiceMuteRequest struct {
	SelfMute bool `json:"self_mute"`
	SelfDeaf bool `json:"self_deaf"`
}

type VoiceQuality struct {
	GuildID   uuid.UUID `json:"guild_id"`
	ChannelID uuid.UUID `json:"channel_id"`
	UserID    uuid.UUID `json:"user_id"`
	Quality   string    `json:"quality"`
	LossPct   float64   `json:"loss_pct"`
	RTTMillis int64     `json:"rtt_ms"`
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
