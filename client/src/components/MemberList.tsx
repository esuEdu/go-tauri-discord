import { useEffect, useState } from "react";
import { emptySession, session, type SessionState } from "../session";
import type { Guild } from "../types/events.gen";

function byName(a: string, b: string): number {
  return session.nameOf(a).localeCompare(session.nameOf(b)) || a.localeCompare(b);
}

export function MemberList({ guild, selfID }: { guild: Guild | null; selfID: string }) {
  const [people, setPeople] = useState<SessionState>(emptySession);

  useEffect(() => session.onChange(setPeople), []);

  if (!guild) return <aside className="members" />;

  const here = people.membersByGuild[guild.id] ?? [];
  const online = here.filter((id) => people.online[id]).sort(byName);
  const offline = here.filter((id) => !people.online[id]).sort(byName);

  return (
    <aside className="members">
      <div className="members-head">{here.length} members</div>

      {here.length === 0 && <div className="muted">Nobody here but you.</div>}

      {online.length > 0 && <div className="members-group muted">Online — {online.length}</div>}
      {online.map((id) => (
        <div key={id} className="member">
          <span className="dot ready" />
          <span className="channel-name">
            {session.nameOf(id)}
            {id === selfID && <span className="muted"> (you)</span>}
          </span>
        </div>
      ))}

      {offline.length > 0 && <div className="members-group muted">Offline — {offline.length}</div>}
      {offline.map((id) => (
        <div key={id} className="member away">
          <span className="dot closed" />
          <span className="channel-name">{session.nameOf(id)}</span>
        </div>
      ))}
    </aside>
  );
}
