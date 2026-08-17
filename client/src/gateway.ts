import {
  OpDispatch,
  OpHeartbeat,
  OpHeartbeatAck,
  OpHello,
  OpIdentify,
  OpInvalidSession,
  OpResume,
  type EventType,
  type Frame,
  type Hello,
  type Ready,
} from "./types/events.gen";
import { gatewayURL } from "./server";

// Same origin as the page unless explicitly overridden, so a tunnelled or
// deployed host needs no rebuild and no configuration.


export type ConnectionState = "connecting" | "ready" | "reconnecting" | "closed";

type Listener = (payload: unknown) => void;

/**
 * Gateway holds the single websocket and hides reconnection from the UI.
 *
 * The protocol requires the client to track its sequence number: on RESUME the
 * server replays from its buffer, and a frame published between the drain and
 * the replay can legitimately arrive twice. Anything not newer than `seq` is
 * therefore discarded rather than dispatched.
 */
export class Gateway {
  private socket: WebSocket | null = null;
  private token: string | null = null;
  private sessionID: string | null = null;
  private seq = 0;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private attempt = 0;
  private closedByUs = false;

  private listeners = new Map<string, Set<Listener>>();
  private controlListeners = new Map<number, Set<Listener>>();
  private stateListeners = new Set<(s: ConnectionState) => void>();

  on(event: EventType | string, fn: Listener): () => void {
    let set = this.listeners.get(event);
    if (!set) {
      set = new Set();
      this.listeners.set(event, set);
    }
    set.add(fn);
    return () => set!.delete(fn);
  }

  onControl(op: number, fn: Listener): () => void {
    let set = this.controlListeners.get(op);
    if (!set) {
      set = new Set();
      this.controlListeners.set(op, set);
    }
    set.add(fn);
    return () => set!.delete(fn);
  }

  sendRaw(frame: Partial<Frame>) {
    this.send(frame);
  }

  onStateChange(fn: (s: ConnectionState) => void): () => void {
    this.stateListeners.add(fn);
    return () => this.stateListeners.delete(fn);
  }

  private setState(s: ConnectionState) {
    for (const fn of this.stateListeners) fn(s);
  }

  private emit(event: string, payload: unknown) {
    const set = this.listeners.get(event);
    if (!set) return;
    for (const fn of set) fn(payload);
  }

  connect(token: string) {
    this.token = token;
    this.closedByUs = false;
    this.open();
  }

  close() {
    this.closedByUs = true;
    this.clearTimers();
    this.sessionID = null;
    this.seq = 0;
    this.socket?.close();
    this.socket = null;
    this.setState("closed");
  }

  private open() {
    this.setState(this.sessionID ? "reconnecting" : "connecting");

    const socket = new WebSocket(gatewayURL());
    this.socket = socket;

    socket.onmessage = (ev) => this.handle(JSON.parse(ev.data as string) as Frame);
    socket.onerror = () => socket.close();
    socket.onclose = () => {
      this.clearTimers();
      if (this.closedByUs) return;
      this.scheduleReconnect();
    };
  }

  private handle(frame: Frame) {
    switch (frame.op) {
      case OpHello: {
        const hello = frame.d as Hello;
        this.startHeartbeat(hello.heartbeat_interval_ms);
        // A live session id means this is a reconnect, so ask for the frames
        // missed while we were away instead of refetching everything.
        if (this.sessionID) {
          this.send({
            op: OpResume,
            d: { token: this.token, session_id: this.sessionID, seq: this.seq },
          });
          // A successful RESUME produces no acknowledgement — the server just
          // replays — so treat the connection as live here. An INVALID_SESSION
          // is the only signal that it failed, and that path corrects the state.
          this.attempt = 0;
          this.setState("ready");
        } else {
          this.send({ op: OpIdentify, d: { token: this.token } });
        }
        return;
      }

      case OpHeartbeatAck:
        return;

      case OpInvalidSession:
        // The session aged out of the server's replay window, so the optimistic
        // "ready" set when RESUME was sent was wrong. Start over.
        this.sessionID = null;
        this.seq = 0;
        this.setState("connecting");
        this.send({ op: OpIdentify, d: { token: this.token } });
        return;

      default: {
        const set = this.controlListeners.get(frame.op);
        if (set) {
          for (const fn of set) fn(frame.d);
        }
        return;
      }

      case OpDispatch: {
        if (typeof frame.s === "number") {
          if (frame.s <= this.seq) return; // Duplicate from a resume replay.
          this.seq = frame.s;
        }
        if (frame.t === "READY") {
          const ready = frame.d as Ready;
          this.sessionID = ready.session_id;
          this.attempt = 0;
          this.setState("ready");
        }
        if (frame.t) this.emit(frame.t, frame.d);
        return;
      }
    }
  }

  private send(frame: Partial<Frame>) {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(frame));
    }
  }

  private startHeartbeat(intervalMS: number) {
    this.clearHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      this.send({ op: OpHeartbeat });
    }, intervalMS);
  }

  private scheduleReconnect() {
    this.setState("reconnecting");
    // Exponential backoff with jitter, capped at 30s, so a server restart does
    // not turn every client into a retry storm at the same instant.
    const base = Math.min(1000 * 2 ** this.attempt, 30_000);
    const delay = base * (0.5 + Math.random() / 2);
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => this.open(), delay);
  }

  private clearHeartbeat() {
    if (this.heartbeatTimer) clearInterval(this.heartbeatTimer);
    this.heartbeatTimer = null;
  }

  private clearTimers() {
    this.clearHeartbeat();
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }
}

export const gateway = new Gateway();
