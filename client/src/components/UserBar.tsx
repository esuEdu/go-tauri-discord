import { useCallback, useEffect, useState } from "react";
import { useDismiss } from "../dismiss";
import type { ConnectionState } from "../gateway";
import { emptySession, session, type SessionState } from "../session";
import { voice, type ScreenState, type VoiceStatus } from "../voice";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";
import { UserSettings } from "./UserSettings";
import type { Channel, User } from "../types/events.gen";

const emptyScreens: ScreenState = {
  sharing: false,
  sound: false,
  local: null,
  remote: [],
  audible: [],
  dropped: [],
  sizes: {},
  quality: "smooth",
};

const LINK = {
  good: { icon: "wifi-high", tone: "quality-good", says: "Voice connected" },
  fair: { icon: "wifi-medium", tone: "quality-fair", says: "Patchy — you may cut out" },
  poor: { icon: "wifi-low", tone: "quality-bad", says: "Bad — they are barely hearing you" },
};

export function UserBar({
  user,
  connection,
  channels,
  onUserChanged,
  onSignOut,
  onDeleteAccount,
  onOpenChannel,
  watching,
}: {
  user: User;
  connection: ConnectionState;
  channels: Channel[];
  onUserChanged: (u: User) => void;
  onSignOut: () => void;
  onDeleteAccount: () => void;
  onOpenChannel: (c: Channel) => void;
  watching: string | null;
}) {
  const [status, setStatus] = useState<VoiceStatus>("idle");
  const [inChannel, setInChannel] = useState<string | null>(null);
  const [muted, setMuted] = useState(false);
  const [deafened, setDeafened] = useState(false);
  const [screens, setScreens] = useState<ScreenState>(emptyScreens);
  const [people, setPeople] = useState<SessionState>(emptySession);
  const [settingsOpen, setSettingsOpen] = useState(false);

  useEffect(() => session.onChange(setPeople), []);
  useEffect(() => voice.onScreenChange(setScreens), []);
  useEffect(
    () =>
      voice.onStatusChange((next, id) => {
        setStatus(next);
        setInChannel(id);
        setMuted(voice.muted);
        setDeafened(voice.deaf);
      }),
    [],
  );

  const close = useCallback(() => setSettingsOpen(false), []);
  const anchor = useDismiss<HTMLDivElement>(settingsOpen, close);

  const call = inChannel ? (channels.find((c) => c.id === inChannel) ?? null) : null;
  const quality = people.connection[user.id];
  const link = status === "connected" ? (LINK[quality as keyof typeof LINK] ?? LINK.good) : null;

  const doing = screens.sharing
    ? "Showing your screen"
    : watching
      ? `Watching ${session.nameOf(watching)}`
      : link
        ? link.says
        : status === "connecting"
          ? "Connecting…"
          : "Could not connect";

  return (
    <div className="userbar">
      {inChannel && (
        <div className="call-row">
          {link ? (
            <Icon name={link.icon} size={16} className={link.tone} />
          ) : (
            <Icon
              name={status === "failed" ? "wifi-slash" : "wifi-none"}
              size={16}
              className="quality-none"
            />
          )}

          <div className="call-row-body">
            <button
              className="call-row-where link-plain"
              disabled={!call}
              onClick={() => call && onOpenChannel(call)}
            >
              {call?.name ?? "a voice channel"}
            </button>
            <div className={link ? "call-row-what" : "call-row-what muted"}>{doing}</div>
          </div>

          <div className="call-row-actions">
            <button
              className="icon-button"
              title={screens.sharing ? "Stop showing your screen" : "Show your screen"}
              onClick={() =>
                screens.sharing ? void voice.stopScreenShare() : void voice.startScreenShare()
              }
            >
              <Icon name={screens.sharing ? "monitor" : "monitor-arrow-up"} size={16} />
            </button>
            <button
              className="icon-button danger"
              title="Leave the call"
              onClick={() => void voice.leave()}
            >
              <Icon name="phone-x" size={16} />
            </button>
          </div>
        </div>
      )}

      {!inChannel && connection !== "ready" && (
        <div className="call-row">
          <span className={`presence-dot ${connection}`} />
          <div className="call-row-body">
            <div className="call-row-where">
              {connection === "closed" ? "Disconnected" : "Reconnecting"}
            </div>
            <div className="call-row-what muted">
              {connection === "closed"
                ? "Nothing is arriving until it comes back."
                : "Messages will send when it comes back."}
            </div>
          </div>
        </div>
      )}

      <div className="anchor" ref={anchor}>
        <div className="identity-row grow">
          <Avatar name={user.username} imageKey={user.avatar_key} mine />

          <div className="identity-body">
            <div className="identity-name clip">{user.username}</div>
            <div className={inChannel ? "identity-state in-call" : "identity-state"}>
              {inChannel ? "In voice" : connection === "ready" ? "Online" : "Offline"}
            </div>
          </div>

          <div className="identity-actions">
            <button
              className={muted ? "icon-button off" : "icon-button on"}
              disabled={!inChannel}
              title={muted ? "Unmute yourself" : "Mute yourself"}
              onClick={() => {
                setMuted(voice.toggleMute());
                setDeafened(voice.deaf);
              }}
            >
              <Icon name={muted ? "microphone-slash" : "microphone"} size={15} />
            </button>
            <button
              className={deafened ? "icon-button off" : "icon-button on"}
              disabled={!inChannel}
              title={deafened ? "Start hearing again" : "Stop hearing everyone"}
              onClick={() => {
                setDeafened(voice.toggleDeafen());
                setMuted(voice.muted);
              }}
            >
              <Icon name={deafened ? "speaker-simple-slash" : "headphones"} size={15} />
            </button>
            <button
              className={settingsOpen ? "icon-button accent" : "icon-button"}
              aria-expanded={settingsOpen}
              title="Your settings"
              onClick={() => setSettingsOpen(!settingsOpen)}
            >
              <Icon name="gear-six" size={15} />
            </button>
          </div>
        </div>

        {settingsOpen && (
          <UserSettings
            user={user}
            connection={connection}
            onUserChanged={onUserChanged}
            onSignOut={onSignOut}
            onDeleteAccount={() => {
              close();
              onDeleteAccount();
            }}
          />
        )}
      </div>
    </div>
  );
}
