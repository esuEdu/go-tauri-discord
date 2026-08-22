package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	"github.com/esuEdu/go-tauri-discord/internal/guild"
	"github.com/esuEdu/go-tauri-discord/internal/platform/pubsub"
	"github.com/esuEdu/go-tauri-discord/pkg/events"

	"sync"
)

type Gateway struct {
	auth        *auth.Service
	guilds      *guild.Service
	broker      pubsub.Broker
	heartbeat   time.Duration
	origins     []string
	maxSessions int
	voice       VoiceEngine
	reads       ReadStates

	mu       sync.RWMutex
	sessions map[string]*session
	byUser   map[uuid.UUID]map[*session]struct{}
	topics   map[string]*topicRoute
	closed   bool
}

type ReadStates interface {
	ReadStates(ctx context.Context, userID uuid.UUID) ([]events.ReadState, error)
	LatestMessages(ctx context.Context, channelIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error)
}

type VoiceEngine interface {
	Join(channelID, userID uuid.UUID, mayStream bool) error
	Leave(userID uuid.UUID)
	Answer(userID uuid.UUID, sdp webrtc.SessionDescription) error
	AddCandidate(userID uuid.UUID, candidate webrtc.ICECandidateInit) error
	Resync(userID uuid.UUID) error
	SetScreenActive(userID uuid.UUID, active bool) error
	ChannelOf(userID uuid.UUID) (uuid.UUID, bool)
	Participants(channelID uuid.UUID) []uuid.UUID
	Muted(channelID uuid.UUID) map[uuid.UUID]bool
	SetMuted(userID uuid.UUID, muted bool) error
	SetWatching(viewerID, sharerID uuid.UUID, watching bool) error
	Sharers(channelID uuid.UUID) map[uuid.UUID]string
}

type topicRoute struct {
	cancel  func()
	members map[*session]struct{}
}

func New(authSvc *auth.Service, guilds *guild.Service, reads ReadStates, broker pubsub.Broker, heartbeat time.Duration, origins []string, maxSessions int) *Gateway {
	if maxSessions < 1 {
		maxSessions = 1
	}
	return &Gateway{
		auth:        authSvc,
		guilds:      guilds,
		reads:       reads,
		broker:      broker,
		heartbeat:   heartbeat,
		origins:     origins,
		maxSessions: maxSessions,
		sessions:    make(map[string]*session),
		byUser:      make(map[uuid.UUID]map[*session]struct{}),
		topics:      make(map[string]*topicRoute),
	}
}

func (g *Gateway) JoinedGuild(userID, guildID uuid.UUID) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return
	}
	for s := range g.byUser[userID] {
		g.subscribeLocked(pubsub.TopicGuild(guildID), s)
		g.subscribeLocked(pubsub.TopicGuildControl(guildID), s)
	}
}

func (g *Gateway) AttachVoice(engine VoiceEngine) {
	g.voice = engine
}

func (g *Gateway) sessionsFor(userID uuid.UUID) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.byUser[userID])
}

func (g *Gateway) register(s *session, guildIDs []uuid.UUID) {
	topics := make([]string, 0, 2*len(guildIDs)+1)
	topics = append(topics, pubsub.TopicUser(s.userID))
	for _, id := range guildIDs {
		topics = append(topics, pubsub.TopicGuild(id), pubsub.TopicGuildControl(id))
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		s.kill()
		return
	}
	g.sessions[s.id] = s
	if g.byUser[s.userID] == nil {
		g.byUser[s.userID] = make(map[*session]struct{})
	}
	g.byUser[s.userID][s] = struct{}{}
	firstSession := len(g.byUser[s.userID]) == 1

	for _, topic := range topics {
		g.subscribeLocked(topic, s)
	}
	g.mu.Unlock()

	if firstSession {
		g.broadcastPresence(s.userID, "online")
	}
}

func (g *Gateway) unregister(s *session) {
	g.mu.Lock()
	delete(g.sessions, s.id)
	if set, ok := g.byUser[s.userID]; ok {
		delete(set, s)
		if len(set) == 0 {
			delete(g.byUser, s.userID)
		}
	}
	lastSession := len(g.byUser[s.userID]) == 0

	s.mu.Lock()
	topics := make([]string, 0, len(s.topics))
	for topic := range s.topics {
		topics = append(topics, topic)
	}
	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
	s.mu.Unlock()

	for _, topic := range topics {
		g.unsubscribeLocked(topic, s)
	}
	g.mu.Unlock()

	s.kill()
	if lastSession {
		g.leaveVoice(s.userID)
		g.broadcastPresence(s.userID, "offline")
	}
}

func (g *Gateway) DisconnectUser(userID uuid.UUID) {
	g.mu.RLock()
	sessions := make([]*session, 0, len(g.byUser[userID]))
	for s := range g.byUser[userID] {
		sessions = append(sessions, s)
	}
	g.mu.RUnlock()

	for _, s := range sessions {
		g.unregister(s)
	}
}

func (g *Gateway) subscribeLocked(topic string, s *session) {
	route, ok := g.topics[topic]
	if !ok {
		ch, cancel := g.broker.Subscribe(topic)
		route = &topicRoute{cancel: cancel, members: make(map[*session]struct{})}
		g.topics[topic] = route
		go g.fanout(topic, ch)
	}
	route.members[s] = struct{}{}

	s.mu.Lock()
	s.topics[topic] = struct{}{}
	s.mu.Unlock()
}

func (g *Gateway) unsubscribeLocked(topic string, s *session) {
	route, ok := g.topics[topic]
	if !ok {
		return
	}
	delete(route.members, s)
	if len(route.members) == 0 {

		delete(g.topics, topic)
		route.cancel()
	}

	s.mu.Lock()
	delete(s.topics, topic)
	s.mu.Unlock()
}

func (g *Gateway) fanout(topic string, ch <-chan []byte) {
	guildID, isControl := pubsub.ControlGuild(topic)

	for raw := range ch {
		if isControl {
			g.refreshVisibility(topic, guildID)
			continue
		}

		channelID, scoped := scopedChannel(raw)

		g.mu.RLock()
		if route, ok := g.topics[topic]; ok {
			for s := range route.members {
				if scoped && !s.canSee(channelID) {
					continue
				}
				s.enqueue(raw)
			}
		}
		g.mu.RUnlock()
	}
}

func scopedChannel(raw []byte) (uuid.UUID, bool) {
	var frame struct {
		T events.EventType `json:"t"`
		D struct {
			ChannelID *uuid.UUID `json:"channel_id"`
		} `json:"d"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return uuid.Nil, false
	}
	switch frame.T {
	case events.EventMessageCreate, events.EventMessageUpdate, events.EventMessageDelete,
		events.EventTypingStart, events.EventVoiceStateUpdate, events.EventVoiceScreenUpdate:
		if frame.D.ChannelID == nil {
			return uuid.Nil, false
		}
		return *frame.D.ChannelID, true
	default:
		return uuid.Nil, false
	}
}

func (g *Gateway) refreshVisibility(topic string, guildID uuid.UUID) {
	g.mu.RLock()
	sessions := make([]*session, 0)
	if route, ok := g.topics[topic]; ok {
		for s := range route.members {
			sessions = append(sessions, s)
		}
	}
	g.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hiddenFor := make(map[uuid.UUID][]uuid.UUID, len(sessions))
	for _, s := range sessions {
		hidden, cached := hiddenFor[s.userID]
		if !cached {
			_, resolved, err := g.guilds.PartitionChannels(ctx, s.userID, guildID)
			if err != nil {
				slog.ErrorContext(ctx, "refresh channel visibility",
					"user_id", s.userID, "guild_id", guildID, "error", err)
				continue
			}
			hidden = resolved
			hiddenFor[s.userID] = hidden
		}
		s.hideInGuild(guildID, hidden)
	}
}

func (g *Gateway) resumableSession(sessionID string, userID uuid.UUID) (*session, bool) {
	g.mu.RLock()
	s, ok := g.sessions[sessionID]
	g.mu.RUnlock()
	if !ok || s.userID != userID {
		return nil, false
	}
	select {
	case <-s.dead:

		return nil, false
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected {

		return nil, false
	}
	if s.expiry != nil {
		s.expiry.Stop()
		s.expiry = nil
	}
	s.connected = true
	return s, true
}

func (g *Gateway) detach(s *session) {
	s.mu.Lock()
	if !s.connected {
		s.mu.Unlock()
		return
	}
	s.connected = false
	s.expiry = time.AfterFunc(resumeWindow, func() {
		s.mu.Lock()
		stillGone := !s.connected
		s.mu.Unlock()
		if stillGone {
			g.unregister(s)
		}
	})
	s.mu.Unlock()
}

func (g *Gateway) broadcastPresence(userID uuid.UUID, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	guilds, err := g.guilds.ListForUser(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "presence lookup", "user_id", userID, "error", err)
		return
	}

	frame, err := events.NewDispatch(events.EventPresenceUpdate, events.PresenceUpdate{
		UserID: userID, Status: status,
	})
	if err != nil {
		return
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	for _, gl := range guilds {
		if err := g.broker.Publish(ctx, pubsub.TopicGuild(gl.ID), raw); err != nil {
			slog.ErrorContext(ctx, "publish presence", "guild_id", gl.ID, "error", err)
		}
	}
}

func (g *Gateway) Close() {
	g.mu.Lock()
	g.closed = true
	sessions := make([]*session, 0, len(g.sessions))
	for _, s := range g.sessions {
		sessions = append(sessions, s)
	}
	for topic, route := range g.topics {
		route.cancel()
		delete(g.topics, topic)
	}
	g.sessions = make(map[string]*session)
	g.byUser = make(map[uuid.UUID]map[*session]struct{})
	g.mu.Unlock()

	for _, s := range sessions {
		s.kill()
	}
}

func (g *Gateway) SessionCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.sessions)
}
