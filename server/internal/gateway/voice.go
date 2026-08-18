package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/pubsub"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (g *Gateway) SendOffer(userID uuid.UUID, sdp webrtc.SessionDescription) {
	g.sendToUser(userID, events.Frame{
		Op: events.OpVoiceOffer,
		D: mustJSON(events.SessionDescription{
			Type: sdp.Type.String(),
			SDP:  sdp.SDP,
		}),
	})
}

func (g *Gateway) SendCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit) {
	g.sendToUser(userID, events.Frame{
		Op: events.OpVoiceCandidate,
		D: mustJSON(events.ICECandidate{
			Candidate:        candidate.Candidate,
			SDPMid:           candidate.SDPMid,
			SDPMLineIndex:    candidate.SDPMLineIndex,
			UsernameFragment: candidate.UsernameFragment,
		}),
	})
}

func (g *Gateway) VoiceClosed(userID uuid.UUID) {
	g.leaveVoice(userID)
}

func (g *Gateway) sendToUser(userID uuid.UUID, frame events.Frame) {
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for s := range g.byUser[userID] {
		s.enqueueControl(raw)
	}
}

func (g *Gateway) handleVoiceState(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}

	var payload events.VoiceStateRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if payload.ChannelID == nil {
		g.leaveVoice(sess.userID)
		return
	}

	perms, channel, err := g.guilds.PermissionsIn(ctx, sess.userID, *payload.ChannelID)
	if err != nil {
		return
	}
	if channel.Kind != domain.ChannelVoice {
		return
	}
	if !perms.Has(domain.PermConnect) {
		return
	}

	previous, wasConnected := g.voice.ChannelOf(sess.userID)
	if wasConnected && previous != *payload.ChannelID {
		g.announceVoice(ctx, sess.userID, previous, nil, payload.SelfMute, payload.SelfDeaf)
	}

	if err := g.voice.Join(*payload.ChannelID, sess.userID); err != nil {
		slog.ErrorContext(ctx, "voice join", "user_id", sess.userID, "error", err)
		return
	}

	g.sendExistingParticipants(sess, channel.GuildID, *payload.ChannelID)
	g.announceVoice(ctx, sess.userID, *payload.ChannelID, payload.ChannelID, payload.SelfMute, payload.SelfDeaf)
}

func (g *Gateway) sendExistingParticipants(sess *session, guildID, channelID uuid.UUID) {
	for _, participant := range g.voice.Participants(channelID) {
		if participant == sess.userID {
			continue
		}
		frame, err := events.NewDispatch(events.EventVoiceStateUpdate, events.VoiceStateUpdate{
			GuildID:   guildID,
			ChannelID: &channelID,
			UserID:    participant,
		})
		if err != nil {
			continue
		}
		raw, err := json.Marshal(frame)
		if err != nil {
			continue
		}
		sess.enqueue(raw)
	}
}

func (g *Gateway) handleVoiceAnswer(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.SessionDescription
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if err := g.voice.Answer(sess.userID, webrtc.SessionDescription{
		Type: webrtc.NewSDPType(payload.Type),
		SDP:  payload.SDP,
	}); err != nil {
		slog.Error("voice answer", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) handleVoiceCandidate(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.ICECandidate
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if err := g.voice.AddCandidate(sess.userID, webrtc.ICECandidateInit{
		Candidate:        payload.Candidate,
		SDPMid:           payload.SDPMid,
		SDPMLineIndex:    payload.SDPMLineIndex,
		UsernameFragment: payload.UsernameFragment,
	}); err != nil {
		slog.Error("voice candidate", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) leaveVoice(userID uuid.UUID) {
	if g.voice == nil {
		return
	}
	channelID, connected := g.voice.ChannelOf(userID)
	if !connected {
		return
	}
	g.voice.Leave(userID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	channel, err := g.guilds.Channel(ctx, channelID)
	if err != nil {
		return
	}
	g.publishVoice(ctx, channel.GuildID, events.VoiceStateUpdate{
		GuildID: channel.GuildID,
		UserID:  userID,
	})
}

func (g *Gateway) announceVoice(ctx context.Context, userID, channelID uuid.UUID, target *uuid.UUID, mute, deaf bool) {
	channel, err := g.guilds.Channel(ctx, channelID)
	if err != nil {
		return
	}
	g.publishVoice(ctx, channel.GuildID, events.VoiceStateUpdate{
		GuildID:   channel.GuildID,
		ChannelID: target,
		UserID:    userID,
		SelfMute:  mute,
		SelfDeaf:  deaf,
	})
}

func (g *Gateway) publishVoice(ctx context.Context, guildID uuid.UUID, state events.VoiceStateUpdate) {
	frame, err := events.NewDispatch(events.EventVoiceStateUpdate, state)
	if err != nil {
		return
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	if err := g.broker.Publish(ctx, pubsub.TopicGuild(guildID), raw); err != nil {
		slog.ErrorContext(ctx, "publish voice state", "error", err)
	}
}
