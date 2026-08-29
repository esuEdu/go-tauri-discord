import type { MouseEvent, PointerEvent as ReactPointerEvent } from "react";
import { Icon } from "./Icon";

export function ChannelRow({
  kind,
  name,
  state = "idle",
  count,
  lifted,
  onClick,
  onContextMenu,
  ...drag
}: {
  kind: "text" | "voice";
  name: string;
  state?: "idle" | "active" | "unread" | "busy";
  count?: number;
  lifted?: boolean;
  onClick?: () => void;
  onContextMenu?: (event: MouseEvent<HTMLButtonElement>) => void;
  draggable?: boolean;
  onDragStart?: (event: React.DragEvent) => void;
  onPointerDown?: (event: ReactPointerEvent<HTMLElement>) => void;
  onPointerMove?: (event: ReactPointerEvent<HTMLElement>) => void;
  onPointerUp?: (event: ReactPointerEvent<HTMLElement>) => void;
  onPointerCancel?: () => void;
}) {
  return (
    <button
      type="button"
      className="channel-row"
      data-state={state}
      data-lifted={lifted}
      onClick={onClick}
      onContextMenu={onContextMenu}
      {...drag}
    >
      <Icon name={kind === "text" ? "hash" : "speaker-high"} size={15} />
      <span className="channel-row-name">{name}</span>
      {state === "unread" && <span className="channel-row-dot" />}
      {state === "busy" && count !== undefined && (
        <span className="channel-row-count">{count}</span>
      )}
    </button>
  );
}
