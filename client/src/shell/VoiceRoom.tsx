import { useEffect, useLayoutEffect, useRef, useState, type MouseEvent } from "react";
import type { SessionState } from "../session";
import type { Channel } from "../types/events.gen";
import { Avatar, initialsOf } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { LiveBadge } from "../ui/LiveBadge";
import { knownTint, tintFor, tintOf } from "../tint";
import { type ScreenState } from "../voice";
import { VoiceControls } from "./VoiceControls";

const TILE_ASPECT = 5 / 3;
const GAP = 6;
const MAX_TILE_WIDTH = 314;

export function VoiceRoom({
  channel,
  inCall,
  state,
  nameFor,
  meID,
  speaking,
  screens,
  muted,
  deafened,
  avatarURL,
  onToggleMute,
  onToggleDeafen,
  onHangUp,
  onGoLive,
  onStopSharing,
  onWatch,
  onOpenMember,
}: {
  channel: Channel;
  inCall: string[];
  state: SessionState;
  nameFor: (id: string) => string;
  meID: string;
  speaking: Record<string, boolean>;
  screens: ScreenState;
  muted: boolean;
  deafened: boolean;
  avatarURL: (id: string) => string | null;
  onToggleMute: () => void;
  onToggleDeafen: () => void;
  onHangUp: () => void;
  onGoLive: () => void;
  onStopSharing: () => void;
  onWatch: (userID: string) => void;
  onOpenMember: (userID: string, event: MouseEvent<HTMLElement>) => void;
}) {
  const stage = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ width: 0, height: 0 });
  useLayoutEffect(() => {
    const node = stage.current;
    if (!node) return;
    const watcher = new ResizeObserver(([entry]) => {
      setBox({
        width: entry.contentRect.width,
        height: entry.contentRect.height,
      });
    });
    watcher.observe(node);
    return () => watcher.disconnect();
  }, []);

  const [tints, setTints] = useState<Record<string, string>>({});

  const pictures = inCall.map(avatarURL).filter(Boolean) as string[];
  useEffect(() => {
    let dropped = false;
    for (const url of pictures) {
      if (knownTint(url) !== undefined) continue;
      void tintOf(url).then((colour) => {
        if (dropped || !colour) return;
        setTints((was) => (was[url] === colour ? was : { ...was, [url]: colour }));
      });
    }
    return () => {
      dropped = true;
    };
  }, [pictures.join(" ")]);

  const live = new Set(screens.remote.map((screen) => screen.userID).filter(Boolean) as string[]);
  if (screens.sharing) live.add(meID);

  const grid = fitTiles(box.width, box.height, inCall.length);

  return (
    <section className="voice-room">
      <header className="room-head">
        <span className="room-name">🔊 {channel.name}</span>
        <span className="room-topic">Voice · {inCall.length} in call</span>
      </header>

      <div className="voice-stage" ref={stage}>
        <div
          className="voice-tiles"
          style={{
            gridTemplateColumns: `repeat(${grid.columns}, ${grid.width}px)`,
            gap: GAP,
          }}
        >
          {inCall.map((id) => {
            const name = nameFor(id);
            const picture = avatarURL(id);
            const avatarSize = Math.round(grid.height * 0.34);
            return (
              <div
                key={id}
                className="voice-tile"
                data-speaking={Boolean(speaking[id])}
                data-live={live.has(id)}
                style={{
                  width: grid.width,
                  height: grid.height,
                  background:
                    (picture ? (tints[picture] ?? knownTint(picture)) : null) ?? tintFor(id),
                }}
                onContextMenu={(event) => onOpenMember(id, event)}
              >
                <span
                  className="voice-tile-initials"
                  style={{
                    width: avatarSize,
                    height: avatarSize,
                    fontSize: Math.round(avatarSize * 0.32),
                  }}
                >
                  {picture ? (
                    <img className="voice-tile-picture" src={picture} alt="" />
                  ) : (
                    initialsOf(name)
                  )}
                </span>
                {live.has(id) && (
                  <>
                    <span className="voice-tile-scrim" />
                    <span className="glass-chip voice-tile-who">
                      <Avatar name={name} url={picture} size={24} />
                      <span className="voice-tile-who-name">{name}</span>
                      <LiveBadge />
                    </span>
                  </>
                )}
                {live.has(id) && id !== meID && (
                  <button
                    type="button"
                    className="watch-pill"
                    onClick={() => onWatch(id)}
                  >
                    Watch
                  </button>
                )}
                <span className="voice-tile-status">
                  {state.mutedInVoice[id] && (
                    <Icon name="microphone-slash" size={16} tone="bad" />
                  )}
                  {state.deafenedInVoice[id] && (
                    <Icon name="headphones-slash" size={16} tone="bad" />
                  )}
                </span>
              </div>
            );
          })}
        </div>

        <VoiceControls
          screens={screens}
          muted={muted}
          deafened={deafened}
          onToggleMute={onToggleMute}
          onToggleDeafen={onToggleDeafen}
          onHangUp={onHangUp}
          onGoLive={onGoLive}
          onStopSharing={onStopSharing}
        />
      </div>
    </section>
  );
}

function fitTiles(width: number, height: number, count: number) {
  if (count === 0 || width <= 0 || height <= 0) {
    return { columns: 1, width: 0, height: 0 };
  }
  let best = { columns: 1, width: 0, height: 0 };
  for (let rows = 1; rows <= count; rows++) {
    const columns = Math.ceil(count / rows);
    let tileWidth = (width - (columns - 1) * GAP) / columns;
    let tileHeight = tileWidth / TILE_ASPECT;
    const needed = tileHeight * rows + (rows - 1) * GAP;
    if (needed > height) {
      tileHeight = (height - (rows - 1) * GAP) / rows;
      tileWidth = tileHeight * TILE_ASPECT;
    }
    if (tileWidth > best.width) {
      const capped = Math.min(tileWidth, MAX_TILE_WIDTH);
      best = {
        columns,
        width: Math.floor(capped),
        height: Math.floor(capped / TILE_ASPECT),
      };
    }
  }
  return best;
}
