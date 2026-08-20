package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/auth"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/guild"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

const (
	readLimit        = 32 << 10
	handshakeTimeout = 10 * time.Second
	writeTimeout     = 10 * time.Second
)

func (g *Gateway) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: g.origins,

			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			slog.WarnContext(r.Context(), "websocket upgrade failed", "error", err)
			return
		}
		conn.SetReadLimit(readLimit)
		defer conn.CloseNow()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		if err := writeFrame(ctx, conn, events.Frame{
			Op: events.OpHello,
			D:  mustJSON(events.Hello{HeartbeatIntervalMS: int(g.heartbeat.Milliseconds())}),
		}); err != nil {
			return
		}

		sess, err := g.handshake(ctx, conn)
		if err != nil {
			closeWith(conn, websocket.StatusPolicyViolation, err.Error())
			return
		}

		defer g.detach(sess)

		go g.writePump(ctx, cancel, conn, sess)
		g.readPump(ctx, conn, sess)
	}
}

func (g *Gateway) handshake(ctx context.Context, conn *websocket.Conn) (*session, error) {
	hsCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	var frame events.Frame
	if err := readFrame(hsCtx, conn, &frame); err != nil {
		return nil, errors.New("handshake timed out")
	}

	switch frame.Op {
	case events.OpIdentify:
		var payload events.Identify
		if err := json.Unmarshal(frame.D, &payload); err != nil {
			return nil, errors.New("malformed identify payload")
		}
		user, err := g.auth.Authenticate(hsCtx, payload.Token)
		if err != nil {
			return nil, errors.New("authentication failed")
		}

		if g.sessionsFor(user.ID) >= g.maxSessions {
			return nil, errors.New("too many concurrent sessions for this account")
		}

		guilds, err := g.guilds.ListForUser(hsCtx, user.ID)
		if err != nil {
			return nil, errors.New("could not load guilds")
		}
		guildIDs := make([]uuid.UUID, len(guilds))
		for i, gl := range guilds {
			guildIDs[i] = gl.ID
		}

		sess := newSession(user)

		if err := g.queueReady(hsCtx, sess, guilds); err != nil {
			return nil, errors.New("could not build ready payload")
		}
		g.register(sess, guildIDs)
		return sess, nil

	case events.OpResume:
		var payload events.Resume
		if err := json.Unmarshal(frame.D, &payload); err != nil {
			return nil, errors.New("malformed resume payload")
		}
		user, err := g.auth.Authenticate(hsCtx, payload.Token)
		if err != nil {
			return nil, errors.New("authentication failed")
		}

		sess, ok := g.resumableSession(payload.SessionID, user.ID)
		if !ok {

			_ = writeFrame(hsCtx, conn, events.Frame{Op: events.OpInvalidSession})
			return nil, errors.New("session is not resumable")
		}

		sess.drainQueued()
		missed, ok := sess.replayAfter(payload.Seq)
		if !ok {
			g.unregister(sess)
			_ = writeFrame(hsCtx, conn, events.Frame{Op: events.OpInvalidSession})
			return nil, errors.New("replay buffer exceeded")
		}
		for _, raw := range missed {
			if err := writeRaw(hsCtx, conn, raw); err != nil {
				return nil, errors.New("replay failed")
			}
		}
		return sess, nil

	default:
		return nil, errors.New("expected identify or resume")
	}
}

func (g *Gateway) queueReady(ctx context.Context, sess *session, guilds []dbgen.Guild) error {
	ready := events.Ready{
		SessionID: sess.id,
		User:      auth.PublicUser(sess.user),
		Guilds:    make([]events.Guild, 0, len(guilds)),
		Channels:  make([]events.Channel, 0),
	}
	for _, gl := range guilds {
		ready.Guilds = append(ready.Guilds, guild.PublicGuild(gl))
		channels, hidden, err := g.guilds.PartitionChannels(ctx, sess.userID, gl.ID)
		if err != nil {
			return err
		}
		for _, ch := range channels {
			ready.Channels = append(ready.Channels, guild.PublicChannel(ch))
		}
		sess.hideInGuild(gl.ID, hidden)
	}

	frame, err := events.NewDispatch(events.EventReady, ready)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	sess.enqueue(raw)
	return nil
}

func (g *Gateway) readPump(ctx context.Context, conn *websocket.Conn, sess *session) {

	idleTimeout := g.heartbeat * 5 / 2

	for {
		readCtx, cancel := context.WithTimeout(ctx, idleTimeout)
		var frame events.Frame
		err := readFrame(readCtx, conn, &frame)
		cancel()
		if err != nil {
			return
		}

		switch frame.Op {
		case events.OpHeartbeat:
			sess.enqueueControl(mustJSON(events.Frame{Op: events.OpHeartbeatAck}))
		case events.OpVoiceState:
			g.handleVoiceState(sess, frame.D)
		case events.OpVoiceAnswer:
			g.handleVoiceAnswer(sess, frame.D)
		case events.OpVoiceCandidate:
			g.handleVoiceCandidate(sess, frame.D)
		case events.OpVoiceResync:
			g.handleVoiceResync(sess)
		case events.OpVoiceScreen:
			g.handleVoiceScreen(sess, frame.D)
		}
	}
}

func (g *Gateway) writePump(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, sess *session) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sess.dead:
			closeWith(conn, websocket.StatusPolicyViolation, "client too slow")
			return
		case raw := <-sess.send:
			if err := writeRaw(ctx, conn, raw); err != nil {
				return
			}
		}
	}
}

func readFrame(ctx context.Context, conn *websocket.Conn, out *events.Frame) error {
	typ, data, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	if typ != websocket.MessageText {
		return errors.New("expected a text frame")
	}
	return json.Unmarshal(data, out)
}

func writeFrame(ctx context.Context, conn *websocket.Conn, frame events.Frame) error {
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return writeRaw(ctx, conn, raw)
}

func writeRaw(ctx context.Context, conn *websocket.Conn, raw []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, raw)
}

func closeWith(conn *websocket.Conn, code websocket.StatusCode, reason string) {
	_ = conn.Close(code, reason)
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic("gateway: marshalling a static frame failed: " + err.Error())
	}
	return raw
}
