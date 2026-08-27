import { useCallback, useEffect, useRef, useState } from "react";
import { canRelay } from "../ice";
import {
  voice,
  MAX_VOLUME,
  SCREEN_QUALITIES,
  type ScreenQualityID,
  noVolumes,
  type ScreenState,
  type VoiceStatus,
  type Speaking,
  type Volumes,
} from "../voice";
import { emptySession, session, type SessionState } from "../session";
import { STREAM, allows } from "../permissions";
import { useDismiss } from "../dismiss";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";
import type { Channel } from "../types/events.gen";

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

type Fit = { width: number; height: number; face: number; gap: number };

const MIN_TILE = 78;

function gapFor(count: number): number {
  if (count <= 2) return 10;
  if (count <= 6) return 8;
  return 7;
}

function fitTiles(box: { width: number; height: number }, count: number): Fit {
  const gap = gapFor(count);
  if (count === 0 || box.width === 0 || box.height === 0) {
    return { width: MIN_TILE, height: MIN_TILE, face: 34, gap };
  }

  let best = { width: MIN_TILE, height: MIN_TILE };

  for (let columns = 1; columns <= count; columns += 1) {
    const rows = Math.ceil(count / columns);
    const width = (box.width - gap * (columns - 1)) / columns;
    const height = (box.height - gap * (rows - 1)) / rows;
    if (width < MIN_TILE || height < MIN_TILE) continue;
    if (width * height > best.width * best.height) best = { width, height };
  }

  const width = Math.floor(best.width);
  const height = Math.floor(best.height);
  const face = Math.min(96, Math.max(28, Math.round(Math.min(width, height) * 0.4)));
  return { width, height, face, gap };
}

function useBox<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [box, setBox] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const watch = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setBox({ width, height });
    });
    watch.observe(element);
    return () => watch.disconnect();
  }, []);

  return [ref, box] as const;
}

function Video({ stream, className }: { stream: MediaStream; className?: string }) {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.srcObject = stream;
    void element.play().catch(() => undefined);
  }, [stream]);

  return <video ref={ref} className={className} autoPlay playsInline muted />;
}

function ScreenStage({
  userID,
  stream,
  size,
  muted,
  onBack,
  onLeave,
}: {
  userID: string;
  stream: MediaStream;
  size: "full" | "half";
  muted: boolean;
  onBack: () => void;
  onLeave: () => void;
}) {
  const [idle, setIdle] = useState(false);
  const [more, setMore] = useState(false);
  const [level, setLevel] = useState(voice.volumeOf(userID, "screen"));
  const [selfMuted, setSelfMuted] = useState(voice.muted);

  const close = useCallback(() => setMore(false), []);
  const anchor = useDismiss<HTMLDivElement>(more, close);

  return (
    <div
      className={idle ? "screen-stage idle" : "screen-stage"}
      onMouseEnter={() => setIdle(false)}
      onMouseLeave={() => {
        setIdle(true);
        setMore(false);
      }}
    >
      <Video stream={stream} />

      <div className="glass glass-chip">
        <Avatar name={session.nameOf(userID)} imageKey={null} />
        <span className="glass-chip-name">{session.nameOf(userID)}</span>
        <span className="live">LIVE</span>
        {muted && <Icon name="speaker-simple-slash" size={14} />}
      </div>

      <div className="glass glass-dock">
        <button
          className={selfMuted ? "dock-button" : "dock-button on"}
          title={selfMuted ? "Unmute yourself" : "Mute yourself"}
          onClick={() => setSelfMuted(voice.toggleMute())}
        >
          <Icon name={selfMuted ? "microphone-slash" : "microphone"} size={17} />
        </button>
        <button className="dock-button" title="Back to the tiles" onClick={onBack}>
          <Icon name="squares-four" size={17} />
        </button>
        <button
          className="dock-button"
          title="Stop receiving this screen"
          onClick={() => {
            voice.watchScreen(userID, false);
            onBack();
          }}
        >
          <Icon name="monitor" size={17} />
        </button>
        <button className="dock-button danger" title="Leave the call" onClick={onLeave}>
          <Icon name="phone-x" size={17} />
        </button>

        <span className="dock-separator" />

        <div className="anchor" ref={anchor}>
          <button
            className={more ? "dock-button on" : "dock-button"}
            aria-expanded={more}
            title="More options"
            onClick={() => setMore(!more)}
          >
            <Icon name="dots-three-vertical" size={17} />
          </button>

          {more && (
            <div className="menu above-right">
              <div className="menu-title">Size</div>
              <button
                className="menu-item"
                onClick={() => voice.watchScreen(userID, true, "full")}
              >
                <Icon name={size === "full" ? "check" : "arrows-out"} size={15} />
                Full
                <span className="menu-item-hint">everything they send</span>
              </button>
              <button
                className="menu-item"
                onClick={() => voice.watchScreen(userID, true, "half")}
              >
                <Icon name={size === "half" ? "check" : "arrows-in"} size={15} />
                Smaller
                <span className="menu-item-hint">half the width, less data</span>
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="glass glass-volume">
        <div className="volume-column">
          <output>{Math.round(level * 100)}%</output>
          <input
            type="range"
            min={0}
            max={MAX_VOLUME * 100}
            step={5}
            value={Math.round(level * 100)}
            aria-label={`Screen volume for ${session.nameOf(userID)}`}
            onChange={(e) => {
              const next = Number(e.target.value) / 100;
              setLevel(next);
              voice.setVolume(userID, "screen", next);
            }}
          />
          <small>{MAX_VOLUME * 100}</small>
        </div>
        <span className="volume-toggle">
          <Icon name="speaker-high" size={17} />
        </span>
      </div>
    </div>
  );
}

export function Voice({
  channel,
  selfID,
  watching,
  onWatch,
}: {
  channel: Channel;
  selfID: string;
  watching: string | null;
  onWatch: (userID: string | null) => void;
}) {
  const [status, setStatus] = useState<VoiceStatus>("idle");
  const [activeChannel, setActiveChannel] = useState<string | null>(null);
  const [screens, setScreens] = useState<ScreenState>(emptyScreens);
  const [volumes, setVolumes] = useState<Volumes>(noVolumes);
  const [people, setPeople] = useState<SessionState>(emptySession);
  const [speaking, setSpeaking] = useState<Speaking>({});
  const [quiet, setQuiet] = useState(false);
  const [pickerRefused, setPickerRefused] = useState(false);

  useEffect(() => session.onChange(setPeople), []);
  useEffect(() => voice.onScreenChange(setScreens), []);
  useEffect(() => voice.onVolumeChange(setVolumes), []);
  useEffect(() => voice.onSpeakingChange(setSpeaking), []);
  useEffect(
    () =>
      voice.onStatusChange((next, id) => {
        setStatus(next);
        setActiveChannel(id);
      }),
    [],
  );

  useEffect(() => onWatch(null), [channel.id, onWatch]);

  const here = activeChannel === channel.id;
  const members = people.inVoice[channel.id] ?? [];
  const mayShare = allows(people.channelAllows[channel.id], STREAM);

  const live = new Set(
    screens.remote.map((s) => s.userID).filter((id): id is string => Boolean(id)),
  );
  const stage = watching ? screens.remote.find((s) => s.userID === watching) : undefined;

  const [tiles, box] = useBox<HTMLDivElement>();
  const fit = fitTiles(box, members.length + (screens.local ? 1 : 0));

  const share = async () => {
    setPickerRefused(false);
    if (!(await voice.startScreenShare())) setPickerRefused(true);
  };

  if (here && stage && stage.userID) {
    return (
      <ScreenStage
        userID={stage.userID}
        stream={stage.stream}
        size={screens.sizes[stage.userID] ?? "full"}
        muted={(volumes.screen[stage.userID] ?? 1) === 0}
        onBack={() => onWatch(null)}
        onLeave={() => {
          onWatch(null);
          void voice.leave();
        }}
      />
    );
  }

  return (
    <>
      <header className="room-head">
        <Icon name="speaker-high" size={17} />
        <span className="room-name">{channel.name}</span>
        <span className="room-topic clip">
          {members.length === 0
            ? "Nobody is here yet."
            : `${members.length} ${members.length === 1 ? "person" : "people"}`}
        </span>
      </header>

      <div className="stage">
        <div className="tiles" ref={tiles} style={{ gap: fit.gap }}>
          {members.length === 0 && !screens.local && (
            <div className="note">Nobody is here yet. Join and the tiles fill up.</div>
          )}

          {screens.local && (
            <div
              className="tile screen"
              style={{ width: fit.width, height: fit.height }}
              key="local"
            >
              <Video stream={screens.local} />
              <span className="tile-corner">
                <span className="clip">Your screen</span>
                {!screens.sound && <Icon name="speaker-simple-slash" size={13} />}
              </span>
            </div>
          )}

          {members.map((id) => {
            const classes = ["tile"];
            if (speaking[id]) classes.push("speaking");
            if (id === selfID) classes.push("mine");
            const watchable = live.has(id);

            return (
              <button
                key={id}
                className={classes.join(" ")}
                style={
                  {
                    width: fit.width,
                    height: fit.height,
                    "--face": `${fit.face}px`,
                  } as React.CSSProperties
                }
                disabled={!watchable}
                title={watchable ? `Watch ${session.nameOf(id)}` : undefined}
                onClick={() => {
                  voice.watchScreen(id, true);
                  onWatch(id);
                }}
              >
                <Avatar
                  name={session.nameOf(id)}
                  imageKey={people.avatars[id]}
                  mine={id === selfID}
                />
                <span className="tile-corner">
                  <span className="clip">{id === selfID ? "You" : session.nameOf(id)}</span>
                  {people.mutedInVoice[id] && <Icon name="microphone-slash" size={13} />}
                  {people.deafenedInVoice[id] && <Icon name="speaker-simple-slash" size={13} />}
                </span>
                {watchable && <span className="live tile-live">LIVE</span>}
              </button>
            );
          })}
        </div>

        <div className="call-foot">
          {here ? (
            <>
              {screens.sharing ? (
                <button className="btn btn-quiet" onClick={() => void voice.stopScreenShare()}>
                  <Icon name="monitor" size={15} />
                  Stop showing your screen
                </button>
              ) : (
                <button className="btn btn-primary" disabled={!mayShare} onClick={() => void share()}>
                  <Icon name="monitor-arrow-up" size={15} />
                  Show your screen
                </button>
              )}

              {mayShare && (
                <span className="select-wrap">
                  <select
                    className="input"
                    aria-label="Screen quality"
                    value={screens.quality}
                    onChange={(e) => voice.setScreenQuality(e.target.value as ScreenQualityID)}
                  >
                    {SCREEN_QUALITIES.map((q) => (
                      <option key={q.id} value={q.id}>
                        {q.label}
                      </option>
                    ))}
                  </select>
                  <Icon name="caret-down" size={13} />
                </span>
              )}

              <button className="btn btn-danger" onClick={() => void voice.leave()}>
                <Icon name="phone-x" size={15} />
                Leave
              </button>
            </>
          ) : (
            <>
              <button
                className="switch"
                role="switch"
                aria-checked={quiet}
                onClick={() => setQuiet(!quiet)}
              >
                <span className="switch-track" />
                Join with my microphone off
              </button>
              <button
                className="btn btn-primary"
                onClick={() => {
                  void voice.join(channel.id, selfID);
                  if (quiet && !voice.muted) voice.toggleMute();
                }}
              >
                <Icon name="phone-call" size={15} />
                Join the call
              </button>
            </>
          )}
        </div>

        <div className="call-notes">
          {here && screens.dropped.length > 0 && (
            <div className="banner">
              <Icon name="monitor-play" size={15} />
              <span className="grow">
                Not watching {screens.dropped.map((id) => session.nameOf(id)).join(", ")}.
              </span>
              <span className="banner-actions">
                {screens.dropped.map((id) => (
                  <button key={id} className="link" onClick={() => voice.watchScreen(id, true)}>
                    Watch {session.nameOf(id)} again
                  </button>
                ))}
              </span>
            </div>
          )}

          {here && screens.sharing && !screens.sound && (
            <span className="note">
              Sharing without sound. A browser only captures audio from a tab — reshare, pick the
              tab that is playing, and tick “Also share tab audio”.
            </span>
          )}

          {here && !mayShare && status === "connected" && (
            <span className="note">You cannot show a screen in this channel.</span>
          )}

          {pickerRefused && (
            <span className="note">
              No screen was picked. Some desktop builds cannot capture a screen — try the browser.
            </span>
          )}

          {status === "failed" && (
            <div className="banner bad">
              <Icon name="warning-circle" size={15} />
              <span>
                {voice.reasonForFailure() === "network"
                  ? canRelay()
                    ? "Could not reach the voice server. Something on the way blocked the connection."
                    : "Could not reach the voice server. This network needs a TURN relay to get out, and this server has none configured."
                  : "Could not connect. Check that the microphone permission was granted."}
              </span>
            </div>
          )}
        </div>
      </div>
    </>
  );
}
