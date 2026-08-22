import { useCallback, useEffect, useState } from "react";
import { api } from "../api";
import type { Channel, Guild } from "../types/events.gen";
import type { Invite } from "../api";

interface Props {
  guilds: Guild[];
  channels: Channel[];
  activeGuild: Guild | null;
  activeChannel: Channel | null;
  onSelectGuild: (g: Guild) => void;
  onSelectChannel: (c: Channel) => void;
  onGuildsChanged: () => void;
  unread: Record<string, boolean>;
}

type ChannelGroup = { category: Channel | null; channels: Channel[] };

function groupByCategory(channels: Channel[]): ChannelGroup[] {
  const categories = channels.filter((c) => c.kind === "category");
  const rest = channels.filter((c) => c.kind !== "category");

  const loose = rest.filter((c) => !c.parent_id || !categories.some((k) => k.id === c.parent_id));
  const groups: ChannelGroup[] = loose.length > 0 ? [{ category: null, channels: loose }] : [];

  for (const category of categories) {
    const under = rest.filter((c) => c.parent_id === category.id);
    if (under.length > 0) groups.push({ category, channels: under });
  }
  return groups;
}

export function Sidebar({
  guilds,
  channels,
  activeGuild,
  activeChannel,
  onSelectGuild,
  onSelectChannel,
  onGuildsChanged,
  unread,
}: Props) {
  const grouped = groupByCategory(channels);

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [maxUses, setMaxUses] = useState("");
  const [expiresIn, setExpiresIn] = useState("");
  const [invites, setInvites] = useState<Invite[]>([]);
  const [managing, setManaging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function createGuild(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    try {
      await api.createGuild(name.trim());
      setName("");
      setCreating(false);
      onGuildsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not create");
    }
  }

  async function joinByCode(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = code.trim();
    if (!trimmed) return;
    setError(null);
    try {
      await api.redeemInvite(trimmed);
      setCode("");
      onGuildsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not join");
    }
  }

  const loadInvites = useCallback(async () => {
    if (!activeGuild) return;
    try {
      setInvites(await api.invites(activeGuild.id));
    } catch {
      setInvites([]);
    }
  }, [activeGuild]);

  useEffect(() => {
    setManaging(false);
    setInvites([]);
    setInviteLink(null);
  }, [activeGuild]);

  useEffect(() => {
    if (managing) void loadInvites();
  }, [managing, loadInvites]);

  async function revoke(code: string) {
    setError(null);
    try {
      await api.revokeInvite(code);
      await loadInvites();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not revoke");
    }
  }

  async function makeInvite() {
    if (!activeGuild) return;
    setError(null);
    try {
      const uses = Number(maxUses);
      const hours = Number(expiresIn);
      const invite = await api.createInvite(
        activeGuild.id,
        uses > 0 ? uses : undefined,
        hours > 0 ? hours : undefined,
      );
      if (managing) void loadInvites();
      const link = `${location.origin}/?invite=${invite.code}`;
      setInviteLink(link);
      try {
        await navigator.clipboard.writeText(link);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      } catch {
        setCopied(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not create invite");
    }
  }

  return (
    <>
      <nav className="guilds">
        {guilds.map((g) => (
          <button
            key={g.id}
            className={g.id === activeGuild?.id ? "guild active" : "guild"}
            title={g.name}
            onClick={() => onSelectGuild(g)}
          >
            {g.name.slice(0, 2).toUpperCase()}
          </button>
        ))}
        <button className="guild add" title="New server" onClick={() => setCreating(!creating)}>
          +
        </button>
      </nav>

      <aside className="channels">
        {creating && (
          <form className="inline-form" onSubmit={createGuild}>
            <input
              placeholder="server name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
            <button type="submit">Create</button>
          </form>
        )}

        <div className="channels-head">{activeGuild?.name ?? "No server"}</div>

        {grouped.map((group) => (
          <div key={group.category?.id ?? "loose"} className="channel-group">
            {group.category && (
              <div className="category">{group.category.name.toUpperCase()}</div>
            )}
            {group.channels.map((c) => (
              <button
                key={c.id}
                className={c.id === activeChannel?.id ? "channel active" : "channel"}
                onClick={() => onSelectChannel(c)}
              >
                <span className="channel-name">
                  {c.kind === "voice" ? "🔊" : "#"} {c.name}
                </span>
                {unread[c.id] && c.id !== activeChannel?.id && (
                  <span className="unread" aria-label="unread messages" />
                )}
              </button>
            ))}
          </div>
        ))}

        <div className="join-box">
          {activeGuild && (
            <>
              <div className="invite-limits">
                <input
                  type="number"
                  min={1}
                  placeholder="uses"
                  aria-label="Maximum uses, blank for unlimited"
                  value={maxUses}
                  onChange={(e) => setMaxUses(e.target.value)}
                />
                <input
                  type="number"
                  min={1}
                  placeholder="hours"
                  aria-label="Expires after, blank for never"
                  value={expiresIn}
                  onChange={(e) => setExpiresIn(e.target.value)}
                />
              </div>
              <button className="invite-button" onClick={() => void makeInvite()}>
                {copied ? "Link copied" : "Invite a friend"}
              </button>
              {inviteLink && (
                <input className="invite-link" readOnly value={inviteLink} onFocus={(e) => e.target.select()} />
              )}

              <button className="link" onClick={() => setManaging(!managing)}>
                {managing ? "Hide invites" : "Manage invites"}
              </button>

              {managing && (
                <div className="invite-list">
                  {invites.length === 0 && <div className="muted">No invites yet.</div>}
                  {invites.map((invite) => (
                    <div key={invite.code} className="invite-row">
                      <code>{invite.code}</code>
                      <span className="muted">
                        {invite.uses}
                        {invite.max_uses ? `/${invite.max_uses}` : ""} used
                        {invite.expires_at
                          ? `, until ${new Date(invite.expires_at).toLocaleDateString()}`
                          : ""}
                      </span>
                      <button className="link danger" onClick={() => void revoke(invite.code)}>
                        Revoke
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}

          <form className="inline-form" onSubmit={joinByCode}>
            <input
              placeholder="invite code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
            <button type="submit">Join</button>
          </form>

          {error && <div className="error inline">{error}</div>}
        </div>
      </aside>
    </>
  );
}
