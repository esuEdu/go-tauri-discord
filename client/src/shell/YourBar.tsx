import { useEffect, useRef, useState } from "react";
import { useDismiss } from "../dismiss";
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
  chosen,
  saying,
  onChooseStatus,
  onSay,
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
  chosen: string;
  saying: string | null;
  onChooseStatus: (status: string) => void;
  onSay: (text: string) => void;
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
        <StatusButton
          me={me}
          meAvatarURL={meAvatarURL}
          presence={presence}
          chosen={chosen}
          saying={saying}
          onChooseStatus={onChooseStatus}
          onSay={onSay}
        />
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

const CHOICES: { value: string; label: string; hint: string }[] = [
  { value: "online", label: "Online", hint: "Here and available" },
  { value: "away", label: "Away", hint: "Around, but not at the keyboard" },
  { value: "busy", label: "Busy", hint: "Here, and would rather not be interrupted" },
  { value: "invisible", label: "Invisible", hint: "You look offline to everybody else" },
];

function StatusButton({
  me,
  meAvatarURL,
  presence,
  chosen,
  saying,
  onChooseStatus,
  onSay,
}: {
  me: string;
  meAvatarURL: string | null;
  presence: string;
  chosen: string;
  saying: string | null;
  onChooseStatus: (status: string) => void;
  onSay: (text: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(saying ?? "");
  const anchor = useDismiss<HTMLSpanElement>(open, () => setOpen(false));

  useEffect(() => {
    if (!open) setDraft(saying ?? "");
  }, [open, saying]);

  const dot = chosen === "invisible" ? "offline" : chosen;

  return (
    <span className="status-anchor" ref={anchor}>
      <button
        type="button"
        className="bar-you-button"
        aria-expanded={open}
        aria-label="Your status"
        onClick={() => setOpen((was) => !was)}
      >
        <span className="avatar-slot">
          <Avatar name={me} url={meAvatarURL} size={34} tone="accent" />
          <span className="presence-dot" data-status={dot} />
        </span>
        <span className="bar-text">
          <span className="bar-you-name">{me}</span>
          <span className="bar-status">{saying ?? presence}</span>
        </span>
      </button>

      {open && (
        <div className="status-popover">
          <span className="status-kicker">Status</span>
          {CHOICES.map((choice) => (
            <button
              key={choice.value}
              type="button"
              className="status-choice"
              data-chosen={choice.value === chosen}
              onClick={() => {
                onChooseStatus(choice.value);
                setOpen(false);
              }}
            >
              <span
                className="presence-dot"
                data-status={choice.value === "invisible" ? "offline" : choice.value}
              />
              <span className="status-choice-text">
                <span className="status-choice-label">{choice.label}</span>
                <span className="status-choice-hint">{choice.hint}</span>
              </span>
            </button>
          ))}

          <span className="status-kicker">Saying</span>
          <div className="status-saying">
            <input
              className="status-input"
              value={draft}
              maxLength={128}
              placeholder="What are you up to?"
              aria-label="Custom status"
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key !== "Enter") return;
                onSay(draft);
                setOpen(false);
              }}
            />
            <button
              type="button"
              className="status-save"
              onClick={() => {
                onSay(draft);
                setOpen(false);
              }}
            >
              Save
            </button>
          </div>
          {saying && (
            <button
              type="button"
              className="status-clear"
              onClick={() => {
                onSay("");
                setOpen(false);
              }}
            >
              Clear it
            </button>
          )}
        </div>
      )}
    </span>
  );
}
