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
  const [joinID, setJoinID] = useState("");
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

  async function joinGuild(e: React.FormEvent) {
    e.preventDefault();
    const id = joinID.trim();
    if (!id) return;
    setError(null);
    try {
      await api.joinGuild(id);
      setJoinID("");
      onGuildsChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not join");
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
          {/* Until invites land (issue #2), joining means pasting a guild id. */}
          <form className="inline-form" onSubmit={joinGuild}>
            <input
              placeholder="join by server id"
              value={joinID}
              onChange={(e) => setJoinID(e.target.value)}
            />
            <button type="submit">Join</button>
          </form>
          {activeGuild && (
            <button
              className="link copy-id"
              onClick={() => void navigator.clipboard.writeText(activeGuild.id)}
            >
              Copy this server's id
            </button>
          )}
          {error && <div className="error inline">{error}</div>}
        </div>
      </aside>
    </>
  );
}
