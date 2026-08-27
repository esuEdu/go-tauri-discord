import { useCallback, useEffect, useState } from "react";
import { BAN_MEMBERS, KICK_MEMBERS, allows } from "../permissions";
import { useDismiss } from "../dismiss";
import { emptySession, session, type SessionState } from "../session";
import type { Guild } from "../types/events.gen";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";
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

  const close = useCallback(() => setSelected(null), []);
  const anchor = useDismiss<HTMLDivElement>(selected !== null, close);

  if (!guild) return null;

  const held = people.guildAllows[guild.id];
  const mayKick = allows(held, KICK_MEMBERS);
  const mayBan = allows(held, BAN_MEMBERS);

  const here = people.membersByGuild[guild.id] ?? [];
  const online = here.filter((id) => people.online[id]).sort(byName);
  const offline = here.filter((id) => !people.online[id]).sort(byName);

  const removable = (id: string) =>
    (mayKick || mayBan) && id !== selfID && id !== guild.owner_id;

  function person(id: string, away: boolean) {
    const tag = people.tags[id];
    const label = (
      <>
        <span className="member-face">
          <Avatar name={session.nameOf(id)} imageKey={people.avatars[id]} mine={id === selfID} />
          {!away && <span className="presence" />}
        </span>
        <span className="member-label">
          <span className="member-name">{session.nameOf(id)}</span>
          {tag && <span className="member-tag">#{tag}</span>}
        </span>
      </>
    );

    if (!removable(id)) {
      return (
        <div key={id} className={away ? "member away" : "member"}>
          {label}
        </div>
      );
    }

    return (
      <div key={id} className="anchor" ref={selected === id ? anchor : undefined}>
        <button
          className={away ? "member away grow" : "member grow"}
          aria-expanded={selected === id}
          onClick={() => setSelected(selected === id ? null : id)}
        >
          {label}
        </button>

        {selected === id && (
          <div className="menu above-right">
            <div className="menu-title">{session.labelOf(id)}</div>
            {mayKick && (
              <button
                className="menu-item"
                onClick={() => {
                  setSelected(null);
                  setRemoving({ userID: id, ban: false });
                }}
              >
                <Icon name="user-minus" size={15} />
                Remove from the server
              </button>
            )}
            {mayBan && (
              <button
                className="menu-item"
                onClick={() => {
                  setSelected(null);
                  setRemoving({ userID: id, ban: true });
                }}
              >
                <Icon name="prohibit" size={15} />
                Ban them
              </button>
            )}
          </div>
        )}
      </div>
    );
  }

  return (
    <aside className="members" aria-label="Members">
      {removing && (
        <RemoveMember
          guild={guild}
          userID={removing.userID}
          name={session.labelOf(removing.userID)}
          ban={removing.ban}
          onClose={() => setRemoving(null)}
        />
      )}

      {here.length === 0 && <div className="note">Nobody here but you.</div>}

      {online.length > 0 && <div className="kicker members-group">Online — {online.length}</div>}
      {online.map((id) => person(id, false))}

      {offline.length > 0 && <div className="kicker members-group">Offline — {offline.length}</div>}
      {offline.map((id) => person(id, true))}
    </aside>
  );
}
