package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/platform/pubsub"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type standing struct {
	public events.UserID
	chosen domain.Status
	custom *string
	idle   bool
}

func (s *session) standing() standing {
	s.mu.Lock()
	defer s.mu.Unlock()
	return standing{
		public: events.UserID(s.user.PublicID),
		chosen: domain.Status(s.user.Status),
		custom: s.user.CustomStatus,
		idle:   s.idle,
	}
}

func (s *session) account() dbgen.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user
}

func (s *session) publicID() events.UserID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return events.UserID(s.user.PublicID)
}

func (s *session) adopt(user dbgen.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user = user
}

func (s *session) reportIdle(idle bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idle = idle
}

func resolved(who standing, connected bool) events.PresenceUpdate {
	status := domain.Resolve(who.chosen, connected, who.idle)
	text := who.custom
	if status == domain.StatusOffline {
		text = nil
	}
	return events.PresenceUpdate{
		UserID:       who.public,
		Status:       string(status),
		CustomStatus: text,
	}
}

func (g *Gateway) presenceOf(userID uuid.UUID, fallback standing) events.PresenceUpdate {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.presenceOfLocked(userID, fallback)
}

func (g *Gateway) presenceOfLocked(userID uuid.UUID, fallback standing) events.PresenceUpdate {
	here := fallback
	live := 0
	idle := true

	for s := range g.byUser[userID] {
		found := s.standing()
		here = found
		idle = idle && found.idle
		live++
	}

	here.idle = idle
	return resolved(here, live > 0)
}

func (g *Gateway) publishPresence(s *session) {
	presence := g.presenceOf(s.userID, s.standing())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	guilds, err := g.guilds.ListForUser(ctx, s.userID)
	if err != nil {
		slog.ErrorContext(ctx, "presence lookup", "user_id", s.userID, "error", err)
		return
	}

	frame, err := events.NewDispatch(events.EventPresenceUpdate, presence)
	if err != nil {
		return
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	for _, gl := range guilds {
		if err := g.broker.Publish(ctx, pubsub.TopicGuild(gl.ID), raw); err != nil {
			slog.ErrorContext(ctx, "publish presence", "error", err)
		}
	}
}

func (g *Gateway) ProfileChanged(user dbgen.User) {
	g.mu.RLock()
	mine := make([]*session, 0, len(g.byUser[user.ID]))
	for s := range g.byUser[user.ID] {
		mine = append(mine, s)
	}
	g.mu.RUnlock()

	if len(mine) == 0 {
		return
	}
	for _, s := range mine {
		s.adopt(user)
	}

	if frame, err := events.NewDispatch(events.EventSelfUpdate, auth.SelfOf(user)); err == nil {
		if raw, err := json.Marshal(frame); err == nil {
			for _, s := range mine {
				s.enqueue(raw)
			}
		}
	}
	g.publishPresence(mine[0])
}

func (g *Gateway) Statuses(userIDs []uuid.UUID) map[uuid.UUID]events.PresenceUpdate {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make(map[uuid.UUID]events.PresenceUpdate, len(userIDs))
	for _, id := range userIDs {
		out[id] = g.presenceOfLocked(id, standing{chosen: domain.StatusOnline})
	}
	return out
}

func (g *Gateway) handlePresence(sess *session, raw json.RawMessage) {
	var payload events.PresenceRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	was := g.presenceOf(sess.userID, sess.standing())
	sess.reportIdle(payload.Idle)

	if now := g.presenceOf(sess.userID, sess.standing()); now.Status != was.Status {
		g.publishPresence(sess)
	}
}
