import type { MouseEvent, ReactNode } from "react";
import { Avatar } from "./Avatar";

export function MemberRow({
  name,
  tag,
  url,
  presence = "online",
  mine,
  trailing,
  onClick,
  onContextMenu,
}: {
  name: string;
  tag?: string;
  url?: string | null;
  presence?: "online" | "offline";
  mine?: boolean;
  trailing?: ReactNode;
  onClick?: () => void;
  onContextMenu?: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  return (
    <button
      type="button"
      className="member-row"
      data-presence={presence}
      onClick={onClick}
      onContextMenu={onContextMenu}
    >
      <Avatar name={name} url={url} size={28} tone={mine ? "accent" : "neutral"} />
      <span className="member-row-identity">
        <span className="member-row-name">{name}</span>
        {tag && <span className="member-row-tag">{tag}</span>}
      </span>
      {trailing}
    </button>
  );
}
