import { useEffect, useRef, useState } from "react";
import { onDesktop } from "../capture";
import { Toggle } from "../ui/Toggle";
import { Avatar, initialsOf } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { IconButton } from "../ui/IconButton";

export type CallSummary = {
  channelName: string;
  status: string;
  quality: "good" | "fair" | "bad";
  sharing: boolean;
};

export type WatchSummary = {
  userID: string;
  name: string;
  avatarURL: string | null;
};

export function YourBar({
  me,
  meAvatarURL,
  presence,
  call,
  watching,
  muted,
  deafened,
  onToggleMute,
  onToggleDeafen,
  onOpenSettings,
  onToggleShare,
  onHangUp,
  suppressing,
  onSuppression,
  onStopWatching,
}: {
  me: string;
  meAvatarURL: string | null;
  presence: string;
  call: CallSummary | null;
  watching: WatchSummary | null;
  muted: boolean;
  deafened: boolean;
  onToggleMute: () => void;
  onToggleDeafen: () => void;
  onOpenSettings: () => void;
  onToggleShare: () => void;
  onHangUp: () => void;
  suppressing: boolean;
  onSuppression: (on: boolean) => void;
  onStopWatching: () => void;
}) {
  return (
    <div className="your-bar">
      {watching && (
        <div className="watching-row">
          <span className="watching-avatar">
            <span className="watching-avatar-inner">
              {watching.avatarURL ? (
                <img src={watching.avatarURL} alt="" />
              ) : (
                initialsOf(watching.name)
              )}
            </span>
          </span>
          <div className="watching-text">
            <span className="watching-who">Watching {watching.name}</span>
            <span className="watching-kind">Screen share</span>
          </div>
          <span className="watching-spacer" />
          <button type="button" className="stop-button" onClick={onStopWatching}>
            Stop
          </button>
        </div>
      )}

      {call && (
        <div className="bar-call">
          <Icon name="wifi-high" size={16} className={`quality-${call.quality}`} />
          <div className="bar-text">
            <span className="bar-title">{call.channelName}</span>
            <span className="bar-status">{call.status}</span>
          </div>
          <div className="bar-actions">
            <NoiseButton suppressing={suppressing} onSuppression={onSuppression} />
            {onDesktop() && (
              <button
                type="button"
                className="bar-icon"
                aria-label={call.sharing ? "Stop sharing your screen" : "Share your screen"}
                title={call.sharing ? "Stop sharing your screen" : "Share your screen"}
                data-active={call.sharing}
                onClick={onToggleShare}
              >
                <Icon name={call.sharing ? "monitor-x" : "monitor-arrow-up"} size={16} />
              </button>
            )}
            <button
              type="button"
              className="bar-icon bar-icon-bad"
              aria-label="Leave the call"
              title="Leave the call"
              onClick={onHangUp}
            >
              <Icon name="phone-x" size={16} />
            </button>
          </div>
        </div>
      )}

      <div className="bar-you">
        <Avatar name={me} url={meAvatarURL} size={34} tone="accent" />
        <div className="bar-text">
          <span className="bar-you-name">{me}</span>
          <span className="bar-status">{presence}</span>
        </div>
        <div className="bar-buttons">
          <IconButton
            name={muted ? "microphone-slash" : "microphone"}
            state={muted ? "off" : "on"}
            label={muted ? "Unmute" : "Mute"}
            onClick={onToggleMute}
          />
          <IconButton
            name={deafened ? "headphones-slash" : "headphones"}
            state={deafened ? "off" : "on"}
            label={deafened ? "Undeafen" : "Deafen"}
            onClick={onToggleDeafen}
          />
          <IconButton
            name="gear-six"
            state="plain"
            label="Settings"
            onClick={onOpenSettings}
          />
        </div>
      </div>
    </div>
  );
}

function NoiseButton({
  suppressing,
  onSuppression,
}: {
  suppressing: boolean;
  onSuppression: (on: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  const anchor = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (!open) return;
    function away(event: PointerEvent) {
      if (!anchor.current?.contains(event.target as Node)) setOpen(false);
    }
    function escape(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    window.addEventListener("pointerdown", away);
    window.addEventListener("keydown", escape);
    return () => {
      window.removeEventListener("pointerdown", away);
      window.removeEventListener("keydown", escape);
    };
  }, [open]);

  return (
    <span className="noise-anchor" ref={anchor}>
      <button
        type="button"
        className="bar-icon"
        aria-label="Noise suppression"
        title="Noise suppression"
        aria-expanded={open}
        data-active={suppressing}
        onClick={() => setOpen((was) => !was)}
      >
        <Icon name={suppressing ? "waveform" : "waveform-slash"} size={16} />
      </button>

      {open && (
        <div className="noise-popover">
          <div className="noise-head">
            <span className="noise-title">Noise suppression</span>
            <Toggle on={suppressing} label="Noise suppression" onChange={onSuppression} />
          </div>
          <p className="noise-text">
            Keeps your voice and drops the rest — keys, fans, the room behind you.
            Everybody else hears the difference, so try it while you type.
          </p>
          <span className="noise-credit">Powered by RNNoise</span>
        </div>
      )}
    </span>
  );
}
