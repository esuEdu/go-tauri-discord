import { useState } from "react";
import { api } from "../api";
import type { Channel, Guild } from "../types/events.gen";

interface Props {
  guilds: Guild[];
  channels: Channel[];
  activeGuild: Guild | null;
  activeChannel: Channel | null;
  onSelectGuild: (g: Guild) => void;
  onSelectChannel: (c: Channel) => void;
  onGuildsChanged: () => void;
}

export function Sidebar({
  guilds,
  channels,
  activeGuild,
  activeChannel,
  onSelectGuild,
  onSelectChannel,
  onGuildsChanged,
}: Props) {
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
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

  async function makeInvite() {
    if (!activeGuild) return;
    setError(null);
    try {
      const invite = await api.createInvite(activeGuild.id);
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

        {channels
          .filter((c) => c.kind !== "category")
          .map((c) => (
            <button
              key={c.id}
              className={c.id === activeChannel?.id ? "channel active" : "channel"}
              onClick={() => onSelectChannel(c)}
            >
              {c.kind === "voice" ? "🔊" : "#"} {c.name}
            </button>
          ))}

        <div className="join-box">
          {activeGuild && (
            <>
              <button className="invite-button" onClick={() => void makeInvite()}>
                {copied ? "Link copied" : "Invite a friend"}
              </button>
              {inviteLink && (
                <input className="invite-link" readOnly value={inviteLink} onFocus={(e) => e.target.select()} />
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
