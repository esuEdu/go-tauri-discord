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

	if err := g.voice.Join(*payload.ChannelID, sess.userID, perms.Has(domain.PermStream)); err != nil {
		slog.ErrorContext(ctx, "voice join", "user_id", sess.userID, "error", err)
		return
	}

	g.sendExistingParticipants(sess, channel.GuildID, *payload.ChannelID)
	g.announceVoice(ctx, sess.userID, *payload.ChannelID, payload.ChannelID, payload.SelfMute, payload.SelfDeaf)
}

func (g *Gateway) sendExistingParticipants(sess *session, guildID, channelID uuid.UUID) {
	for _, participant := range g.voice.States(channelID) {
		if participant.UserID == sess.userID {
			continue
		}
		frame, err := events.NewDispatch(events.EventVoiceStateUpdate, events.VoiceStateUpdate{
			GuildID:   guildID,
			ChannelID: &channelID,
			UserID:    participant.UserID,
			SelfMute:  participant.Muted,
			SelfDeaf:  participant.Deafened,
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

	for participant, streamID := range g.voice.Sharers(channelID) {
		if participant == sess.userID {
			continue
		}
		frame, err := events.NewDispatch(events.EventVoiceScreenUpdate, events.VoiceScreenUpdate{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    participant,
			StreamID:  streamID,
			Active:    true,
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

func (g *Gateway) handleVoiceResync(sess *session) {
	if g.voice == nil {
		return
	}
	if err := g.voice.Resync(sess.userID); err != nil {
		slog.Error("voice resync", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) handleVoiceMute(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.VoiceMuteRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if err := g.voice.SetMuted(sess.userID, payload.SelfMute); err != nil {
		return
	}
	if err := g.voice.SetDeafened(sess.userID, payload.SelfDeaf); err != nil {
		return
	}

	channelID, connected := g.voice.ChannelOf(sess.userID)
	if !connected {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	channel, err := g.guilds.Channel(ctx, channelID)
	if err != nil {
		return
	}
	g.publishVoice(ctx, channel.GuildID, events.VoiceStateUpdate{
		GuildID:   channel.GuildID,
		ChannelID: &channelID,
		UserID:    sess.userID,
		SelfMute:  payload.SelfMute,
		SelfDeaf:  payload.SelfDeaf,
	})
}

func (g *Gateway) ScreenAnswer(userID uuid.UUID, sdp webrtc.SessionDescription) {
	g.sendToUser(userID, events.Frame{
		Op: events.OpScreenAnswer,
		D:  mustJSON(events.ScreenPublish{SDP: sdp.SDP}),
	})
}

func (g *Gateway) ScreenCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit) {
	g.sendToUser(userID, events.Frame{
		Op: events.OpScreenIce,
		D: mustJSON(events.ICECandidate{
			Candidate:        candidate.Candidate,
			SDPMid:           candidate.SDPMid,
			SDPMLineIndex:    candidate.SDPMLineIndex,
			UsernameFragment: candidate.UsernameFragment,
		}),
	})
}

func (g *Gateway) handleScreenPublish(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.ScreenPublish
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if payload.SDP == "" {
		g.voice.StopPublishing(sess.userID)
		return
	}
	if err := g.voice.PublishScreen(sess.userID, webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer, SDP: payload.SDP,
	}); err != nil {
		slog.Error("screen publish", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) handleScreenIce(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.ICECandidate
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if err := g.voice.PublishCandidate(sess.userID, webrtc.ICECandidateInit{
		Candidate:        payload.Candidate,
		SDPMid:           payload.SDPMid,
		SDPMLineIndex:    payload.SDPMLineIndex,
		UsernameFragment: payload.UsernameFragment,
	}); err != nil {
		slog.Error("screen candidate", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) handleVoiceWatch(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.VoiceWatchRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if err := g.voice.SetWatching(sess.userID, payload.UserID, payload.Watching, payload.Size); err != nil {
		slog.Error("voice watch", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) handleVoiceScreen(sess *session, raw json.RawMessage) {
	if g.voice == nil {
		return
	}
	var payload events.VoiceScreenRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if err := g.voice.SetScreenActive(sess.userID, payload.Active); err != nil {
		slog.Error("voice screen", "user_id", sess.userID, "error", err)
	}
}

func (g *Gateway) ScreenChanged(channelID, userID uuid.UUID, streamID string, active bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	channel, err := g.guilds.Channel(ctx, channelID)
	if err != nil {
		return
	}

	frame, err := events.NewDispatch(events.EventVoiceScreenUpdate, events.VoiceScreenUpdate{
		GuildID:   channel.GuildID,
		ChannelID: channelID,
		UserID:    userID,
		StreamID:  streamID,
		Active:    active,
	})
	if err != nil {
		return
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	if err := g.broker.Publish(ctx, pubsub.TopicGuild(channel.GuildID), raw); err != nil {
		slog.ErrorContext(ctx, "publish screen state", "error", err)
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
