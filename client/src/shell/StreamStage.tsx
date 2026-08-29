import { useEffect, useRef, useState } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { LiveBadge } from "../ui/LiveBadge";
import { MAX_VOLUME, voice, type ScreenState } from "../voice";
import { VoiceControls } from "./VoiceControls";

export function StreamStage({
  name,
  avatarURL,
  stream,
  userID,
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
  name: string;
  avatarURL: string | null;
  stream: MediaStream | null;
  userID: string;
  screens: ScreenState;
  muted: boolean;
  deafened: boolean;
  onToggleMute: () => void;
  onToggleDeafen: () => void;
  onHangUp: () => void;
  onGoLive: () => void;
  onStopSharing: () => void;
  onStopWatching: () => void;
}) {
  const video = useRef<HTMLVideoElement>(null);
  const [level, setLevel] = useState(() => voice.volumeOf(userID, "screen"));

  useEffect(() => {
    if (video.current && stream) video.current.srcObject = stream;
  }, [stream]);

  useEffect(() => {
    setLevel(voice.volumeOf(userID, "screen"));
  }, [userID]);

  const muteThem = level === 0;

  return (
    <section className="stream-stage">
      <video ref={video} className="stream-video" autoPlay playsInline />

      <span className="glass-chip stream-who">
        <Avatar name={name} url={avatarURL} size={24} />
        <span className="stream-who-name">{name}</span>
        <LiveBadge />
        <Icon name={muteThem ? "speaker-slash" : "speaker-high"} size={14} />
      </span>

      <div className="stream-chrome">
        <VoiceControls
          screens={screens}
          muted={muted}
          deafened={deafened}
          onToggleMute={onToggleMute}
          onToggleDeafen={onToggleDeafen}
          onHangUp={onHangUp}
          onGoLive={onGoLive}
          onStopSharing={onStopSharing}
          onStopWatching={onStopWatching}
        />

        <div className="stream-volume glass-chip">
          <span className="stream-volume-value">{Math.round(level * 100)}%</span>
          <input
            type="range"
            className="stream-volume-input"
            min={0}
            max={MAX_VOLUME * 100}
            value={Math.round(level * 100)}
            aria-label={`${name}'s stream volume`}
            style={{ "--fill": `${(level / MAX_VOLUME) * 100}%` } as React.CSSProperties}
            onChange={(event) => {
              const next = Number(event.target.value) / 100;
              setLevel(next);
              voice.setVolume(userID, "screen", next);
            }}
          />
          <span className="stream-volume-max">{MAX_VOLUME * 100}</span>
        </div>

        <button
          type="button"
          className="control stream-volume-button"
          aria-label={muteThem ? `Unmute ${name}` : `Mute ${name}`}
          title={muteThem ? `Unmute ${name}` : `Mute ${name}`}
          onClick={() => {
            const next = muteThem ? 1 : 0;
            setLevel(next);
            voice.setVolume(userID, "screen", next);
          }}
        >
          <Icon name={muteThem ? "speaker-slash" : "speaker-high"} size={17} />
        </button>
      </div>
    </section>
  );
}
