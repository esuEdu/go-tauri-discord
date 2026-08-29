import { useRef, useState } from "react";
import type { Guild } from "../types/events.gen";
import { initialsOf } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

const TILE = 44;
const GAP = 10;
const PAD_TOP = 4;
const PITCH = TILE + GAP;
const THRESHOLD = 5;

type Drag = {
  id: string;
  from: number;
  initials: string;
  url: string | null;
  x: number;
  y: number;
  grabX: number;
  grabY: number;
  lifted: boolean;
};

export function ServerRail({
  guilds,
  activeID,
  iconURL,
  onPick,
  onAdd,
  onReorder,
}: {
  guilds: Guild[];
  activeID: string | null;
  iconURL: (guild: Guild) => string | null;
  onPick: (guild: Guild) => void;
  onAdd: () => void;
  onReorder: (guildIDs: string[]) => void;
}) {
  const rail = useRef<HTMLElement>(null);
  const [drag, setDrag] = useState<Drag | null>(null);
  const [dropAt, setDropAt] = useState<number | null>(null);
  const clickable = useRef(true);

  function slotFor(clientY: number, fallback: number): number {
    const node = rail.current;
    if (!node) return fallback;
    const box = node.getBoundingClientRect();
    const y = clientY - box.top + node.scrollTop - PAD_TOP;
    return Math.max(0, Math.min(guilds.length, Math.round(y / PITCH)));
  }

  function stop() {
    setDrag(null);
    setDropAt(null);
  }

  const lineTop =
    dropAt === null ? 0 : Math.max(1, PAD_TOP + dropAt * PITCH - GAP / 2 - 1.5);

  return (
    <nav className="server-rail" aria-label="Servers" ref={rail}>
      {drag?.lifted && dropAt !== null && (
        <span className="rail-drop-line" style={{ top: lineTop }} />
      )}

      {guilds.map((guild, index) => (
        <button
          key={guild.id}
          type="button"
          className="server-tile"
          data-active={guild.id === activeID}
          data-lifted={drag?.id === guild.id && drag.lifted}
          title={guild.name}
          draggable={false}
          onDragStart={(event) => event.preventDefault()}
          onPointerDown={(event) => {
            if (event.button !== 0) return;
            clickable.current = true;
            event.currentTarget.setPointerCapture(event.pointerId);
            setDrag({
              id: guild.id,
              from: index,
              initials: initialsOf(guild.name),
              url: iconURL(guild),
              x: event.clientX,
              y: event.clientY,
              grabX: event.clientX,
              grabY: event.clientY,
              lifted: false,
            });
          }}
          onPointerMove={(event) => {
            if (!drag || drag.id !== guild.id) return;
            const far =
              Math.abs(event.clientX - drag.grabX) >= THRESHOLD ||
              Math.abs(event.clientY - drag.grabY) >= THRESHOLD;
            if (!drag.lifted && !far) return;
            clickable.current = false;
            setDrag({ ...drag, x: event.clientX, y: event.clientY, lifted: true });
            setDropAt(slotFor(event.clientY, drag.from));
          }}
          onPointerUp={(event) => {
            if (event.currentTarget.hasPointerCapture(event.pointerId)) {
              event.currentTarget.releasePointerCapture(event.pointerId);
            }
            if (drag?.lifted && dropAt !== null) {
              const order = guilds.map((g) => g.id);
              const [held] = order.splice(drag.from, 1);
              order.splice(dropAt > drag.from ? dropAt - 1 : dropAt, 0, held);
              if (order.some((id, at) => id !== guilds[at].id)) onReorder(order);
            }
            stop();
          }}
          onPointerCancel={stop}
          onClick={() => {
            if (!clickable.current) {
              clickable.current = true;
              return;
            }
            onPick(guild);
          }}
        >
          {iconURL(guild) ? (
            <img src={iconURL(guild)!} alt="" draggable={false} />
          ) : (
            initialsOf(guild.name)
          )}
        </button>
      ))}

      <button
        type="button"
        className="server-tile"
        data-add="true"
        title="Add a server"
        aria-label="Add a server"
        onClick={onAdd}
      >
        <Icon name="plus" size={17} />
      </button>

      {drag?.lifted && (
        <span className="rail-ghost" style={{ left: drag.x, top: drag.y }}>
          {drag.url ? <img src={drag.url} alt="" draggable={false} /> : drag.initials}
        </span>
      )}
    </nav>
  );
}
