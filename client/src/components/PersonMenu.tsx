import { useEffect, useState } from "react";
import { session } from "../session";
import { voice, MAX_VOLUME } from "../voice";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";

export function PersonMenu({
  userID,
  live,
  onClose,
}: {
  userID: string;
  live: boolean;
  onClose: () => void;
}) {
  const [level, setLevel] = useState(voice.volumeOf(userID, "voice"));

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="dialog-backdrop" onMouseDown={onClose}>
      <div className="menu floating" role="menu" onMouseDown={(e) => e.stopPropagation()}>
        <div className="menu-person">
          <Avatar name={session.nameOf(userID)} imageKey={null} />
          <span className="choice-name clip">{session.labelOf(userID)}</span>
        </div>

        <button
          className="menu-item"
          disabled={!live}
          onClick={() => {
            voice.watchScreen(userID, true);
            onClose();
          }}
        >
          <Icon name="monitor-play" size={15} />
          Watch their screen
          {!live && <span className="menu-item-hint">not sharing</span>}
        </button>

        <div className="menu-slider">
          <div className="menu-slider-head">
            <span>Their voice, for you</span>
            <b>{Math.round(level * 100)}%</b>
          </div>
          <input
            type="range"
            min={0}
            max={MAX_VOLUME * 100}
            step={5}
            value={Math.round(level * 100)}
            aria-label={`Volume for ${session.nameOf(userID)}`}
            onChange={(e) => {
              const next = Number(e.target.value) / 100;
              setLevel(next);
              voice.setVolume(userID, "voice", next);
            }}
          />
          <span className="field-note">
            0 to 200%, remembered between calls, and they are never told.
          </span>
        </div>
      </div>
    </div>
  );
}
