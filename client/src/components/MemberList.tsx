import { useEffect, useState } from "react";
import { BAN_MEMBERS, KICK_MEMBERS, allows } from "../permissions";
import { emptySession, session, type SessionState } from "../session";
import type { Guild } from "../types/events.gen";
import { RemoveMember } from "./RemoveMember";

function byName(a: string, b: string): number {
  return session.nameOf(a).localeCompare(session.nameOf(b)) || a.localeCompare(b);
}

export function MemberList({ guild, selfID }: { guild: Guild | null; selfID: string }) {
  const [people, setPeople] = useState<SessionState>(emptySession);
  const [selected, setSelected] = useState<string | null>(null);
  const [removing, setRemoving] = useState<{ userID: string; ban: boolean } | null>(null);

  useEffect(() => session.onChange(setPeople), []);
  useEffect(() => setSelected(null), [guild?.id]);

  if (!guild) return <aside className="members" />;

  const held = people.guildAllows[guild.id];
  const mayKick = allows(held, KICK_MEMBERS);
  const mayBan = allows(held, BAN_MEMBERS);

  const here = people.membersByGuild[guild.id] ?? [];
  const online = here.filter((id) => people.online[id]).sort(byName);
  const offline = here.filter((id) => !people.online[id]).sort(byName);

  const removable = (id: string) =>
    (mayKick || mayBan) && id !== selfID && id !== guild.owner_id;

  function person(id: string, away: boolean) {
    const open = selected === id;
    return (
      <div key={id} className={away ? "member away" : "member"}>
        <button
          className="member-name"
          disabled={!removable(id)}
          aria-expanded={removable(id) ? open : undefined}
          onClick={() => setSelected(open ? null : id)}
        >
          <span className={away ? "dot closed" : "dot ready"} />
          <span className="channel-name">
            {session.nameOf(id)}
            {id === selfID && <span className="muted"> (you)</span>}
          </span>
        </button>

        {open && removable(id) && (
          <div className="member-actions">
            {mayKick && (
              <button className="link danger" onClick={() => setRemoving({ userID: id, ban: false })}>
                Kick
              </button>
            )}
            {mayBan && (
              <button className="link danger" onClick={() => setRemoving({ userID: id, ban: true })}>
                Ban
              </button>
            )}
          </div>
        )}
      </div>
    );
  }

  return (
    <aside className="members">
      {removing && (
        <RemoveMember
          guild={guild}
          userID={removing.userID}
          name={session.nameOf(removing.userID)}
          ban={removing.ban}
          onClose={() => {
            setRemoving(null);
            setSelected(null);
          }}
        />
      )}

      <div className="members-head">{here.length} members</div>

      {here.length === 0 && <div className="muted">Nobody here but you.</div>}

      {online.length > 0 && <div className="members-group muted">Online — {online.length}</div>}
      {online.map((id) => person(id, false))}

      {offline.length > 0 && <div className="members-group muted">Offline — {offline.length}</div>}
      {offline.map((id) => person(id, true))}
    </aside>
  );
}
