import { useEffect, useState } from "react";
import {
  chooseMicrophone,
  chosenMicrophone,
  microphones,
  type Microphone,
} from "../audioPrefs";
import { Icon } from "../ui/Icon";
import { SCREEN_QUALITIES, voice, type ScreenState } from "../voice";

export function VoiceControls({
  screens,
  muted,
  deafened,
  onToggleMute,
  onToggleDeafen,
  onHangUp,
  onGoLive,
  onStopSharing,
  onStopWatching,
}: {
  screens: ScreenState;
  muted: boolean;
  deafened: boolean;
  onToggleMute: () => void;
  onToggleDeafen: () => void;
  onHangUp: () => void;
  onGoLive: () => void;
  onStopSharing: () => void;
  onStopWatching?: () => void;
}) {
  const [popover, setPopover] = useState<"devices" | "quality" | "source" | null>(null);
  const [mics, setMics] = useState<Microphone[]>([]);

  useEffect(() => {
    if (popover !== "devices") return;
    void microphones().then(setMics);
  }, [popover]);

  useEffect(() => {
    function away() {
      setPopover(null);
    }
    if (!popover) return;
    window.addEventListener("pointerdown", away);
    return () => window.removeEventListener("pointerdown", away);
  }, [popover]);

  return (
    <>
      <div className="controls" onPointerDown={(event) => event.stopPropagation()}>
        <button
          type="button"
          className="control"
          data-active={!muted}
          aria-label={muted ? "Unmute" : "Mute"}
          title={muted ? "Unmute" : "Mute"}
          onClick={onToggleMute}
        >
          <Icon name={muted ? "microphone-slash" : "microphone"} size={17} />
        </button>
        <button
          type="button"
          className="control"
          aria-label={deafened ? "Undeafen" : "Deafen"}
          title={deafened ? "Undeafen" : "Deafen"}
          onClick={onToggleDeafen}
        >
          <Icon name={deafened ? "headphones-slash" : "headphones"} size={17} />
        </button>
        <span className="control-anchor">
          <button
            type="button"
            className="control"
            aria-label="Choose a microphone"
            title="Choose a microphone"
            onClick={() => setPopover(popover === "devices" ? null : "devices")}
          >
            <Icon name="caret-down" size={17} />
          </button>
          {popover === "devices" && <DevicePicker mics={mics} onPick={() => setPopover(null)} />}
        </span>

        <button
          type="button"
          className="control"
          data-active={screens.sharing}
          aria-label={screens.sharing ? "Stop sharing" : "Share your screen"}
          title={screens.sharing ? "Stop sharing" : "Share your screen"}
          onClick={screens.sharing ? onStopSharing : onGoLive}
        >
          <Icon name={screens.sharing ? "monitor-x" : "monitor-arrow-up"} size={17} />
        </button>
        {screens.sharing && (
          <span className="control-anchor">
            <button
              type="button"
              className="control"
              aria-label="Change what you are sharing"
              title="Change what you are sharing"
              onClick={() => setPopover(popover === "source" ? null : "source")}
            >
              <Icon name="caret-down" size={17} />
            </button>
            {popover === "source" && (
              <SharePicker
                onChange={() => { setPopover(null); onGoLive(); }}
                onStop={() => { setPopover(null); onStopSharing(); }}
              />
            )}
          </span>
        )}

        <button
          type="button"
          className="control"
          data-tone="bad"
          aria-label="Leave the call"
          title="Leave the call"
          onClick={onHangUp}
        >
          <Icon name="phone-x" size={17} />
        </button>

        <span className="control-divider" />
        <span className="control-anchor">
          <button
            type="button"
            className="control"
            aria-label="Video quality"
            title="Video quality"
            onClick={() => setPopover(popover === "quality" ? null : "quality")}
          >
            <Icon name="caret-down" size={17} />
          </button>
          {popover === "quality" && (
            <QualityPicker chosen={screens.quality} onPick={() => setPopover(null)} />
          )}
        </span>

        {onStopWatching && <span className="control-divider" />}
        {onStopWatching && (
          <button
            type="button"
            className="control"
            aria-label="Stop watching"
            title="Stop watching"
            onClick={onStopWatching}
          >
            <Icon name="monitor-x" size={15} />
          </button>
        )}
      </div>
    </>
  );
}

function DevicePicker({ mics, onPick }: { mics: Microphone[]; onPick: () => void }) {
  return (
    <div className="voice-popover" data-width="wide">
      <span className="voice-popover-title">Input device</span>
      <button
        type="button"
        className="voice-option"
        data-active={chosenMicrophone() === null}
        onClick={() => {
          chooseMicrophone(null);
          onPick();
        }}
      >
        <Icon name="microphone" size={16} />
        <span className="voice-option-label">System default</span>
      </button>
      {mics.map((mic) => (
        <button
          key={mic.id}
          type="button"
          className="voice-option"
          data-active={chosenMicrophone() === mic.id}
          onClick={() => {
            chooseMicrophone(mic.id);
            onPick();
          }}
        >
          <Icon name="microphone" size={16} />
          <span className="voice-option-label">{mic.label}</span>
        </button>
      ))}
    </div>
  );
}

function QualityPicker({
  chosen,
  onPick,
}: {
  chosen: ScreenState["quality"];
  onPick: () => void;
}) {
  return (
    <div className="voice-popover">
      <span className="voice-popover-title">Video quality</span>
      {SCREEN_QUALITIES.map((entry) => (
        <button
          key={entry.id}
          type="button"
          className="voice-option"
          data-active={chosen === entry.id}
          onClick={() => {
            voice.setScreenQuality(entry.id);
            onPick();
          }}
        >
          <Icon name="speaker-high" size={16} />
          <span className="voice-option-label">{entry.label}</span>
        </button>
      ))}
    </div>
  );
}

function SharePicker({ onChange, onStop }: { onChange: () => void; onStop: () => void }) {
  return (
    <div className="voice-popover" data-width="wide">
      <span className="voice-popover-title">Sharing</span>
      <button type="button" className="voice-option" onClick={onChange}>
        <Icon name="monitor-arrow-up" size={16} />
        <span className="voice-option-label">Change what you share</span>
      </button>
      <button type="button" className="voice-option" onClick={onStop}>
        <Icon name="monitor-x" size={16} />
        <span className="voice-option-label">Stop sharing</span>
      </button>
    </div>
  );
}
