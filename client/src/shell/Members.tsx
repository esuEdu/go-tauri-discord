import type { MouseEvent } from "react";
import type { SessionState } from "../session";
import { Avatar } from "../ui/Avatar";
import { LiveBadge } from "../ui/LiveBadge";

export function Members({
  ids,
  state,
  nameFor,
  meID,
  live,
  avatarURL,
  onOpenMenu,
}: {
  ids: string[];
  state: SessionState;
  nameFor: (id: string) => string;
  meID: string;
  live: Set<string>;
  avatarURL: (id: string) => string | null;
  onOpenMenu: (userID: string, event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const named = ids.filter((id) => state.names[id]);
  const online = named.filter((id) => state.online[id]);
  const offline = named.filter((id) => !state.online[id]);
  const shared = sharedNames(named, nameFor);

  function group(title: string, list: string[], presence: "online" | "offline") {
    if (!list.length) return null;
    return (
      <div className="members-group">
        <span className="members-kicker">
          {title} — {list.length}
        </span>
        {list.map((id) => (
          <button
            key={id}
            type="button"
            className="member-row"
            data-presence={presence}
            onContextMenu={(event) => onOpenMenu(id, event)}
          >
            <span className="avatar-slot">
              <Avatar
                name={nameFor(id)}
                url={avatarURL(id)}
                size={28}
                tone={id === meID ? "accent" : "neutral"}
              />
              {presence === "online" && <span className="presence-dot" />}
            </span>
            <span className="member-row-identity">
              <span className="member-row-name">{nameFor(id)}</span>
              {shared.has(nameFor(id)) && state.tags[id] && (
                <span className="member-row-tag">#{state.tags[id]}</span>
              )}
            </span>
            {live.has(id) && <LiveBadge />}
          </button>
        ))}
      </div>
    );
  }

  return (
    <aside className="members panel" aria-label="Members">
      {group("Online", online, "online")}
      {group("Offline", offline, "offline")}
    </aside>
  );
}

function sharedNames(ids: string[], nameFor: (id: string) => string): Set<string> {
  const seen = new Map<string, number>();
  for (const id of ids) {
    const name = nameFor(id);
    if (name) seen.set(name, (seen.get(name) ?? 0) + 1);
  }
  const shared = new Set<string>();
  for (const [name, count] of seen) if (count > 1) shared.add(name);
  return shared;
}
