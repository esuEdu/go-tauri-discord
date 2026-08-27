import { useCallback, useEffect, useState } from "react";
import { Avatar } from "./Avatar";
import { api } from "../api";
import { ServerSettings } from "./ServerSettings";
import { emptySession, session, type SessionState } from "../session";
import { BAN_MEMBERS, CREATE_INVITE, MANAGE_CHANNELS, MANAGE_ROLES, allows } from "../permissions";
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

function categoriesIn(channels: Channel[]): Channel[] {
  return channels.filter((c) => c.kind === "category");
}

function inOrder(channels: Channel[]): Channel[] {
  return [...channels].sort((a, b) =>
    a.position === b.position ? a.id.localeCompare(b.id) : a.position - b.position,
  );
}

function groupByCategory(channels: Channel[]): ChannelGroup[] {
  const categories = inOrder(channels.filter((c) => c.kind === "category"));
  const rest = channels.filter((c) => c.kind !== "category");

  const loose = inOrder(
    rest.filter((c) => !c.parent_id || !categories.some((k) => k.id === c.parent_id)),
  );
  const groups: ChannelGroup[] = loose.length > 0 ? [{ category: null, channels: loose }] : [];

  for (const category of categories) {
    groups.push({
      category,
      channels: inOrder(rest.filter((c) => c.parent_id === category.id)),
    });
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

  const move = async (channelID: string, to: number) => {
    try {
      await api.moveChannel(channelID, to);
    } catch {}
  };

  const [people, setPeople] = useState<SessionState>(emptySession);
  useEffect(() => session.onChange(setPeople), []);

  const guildAllows = activeGuild ? people.guildAllows[activeGuild.id] : undefined;
  const mayManageChannels = allows(guildAllows, MANAGE_CHANNELS);
  const mayManageRoles = allows(guildAllows, MANAGE_ROLES);
  const mayBan = allows(guildAllows, BAN_MEMBERS);
  const mayInvite = allows(guildAllows, CREATE_INVITE);

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [addingChannel, setAddingChannel] = useState(false);
  const [channelName, setChannelName] = useState("");
  const [channelKind, setChannelKind] = useState<"text" | "voice" | "category">("text");
  const [channelParent, setChannelParent] = useState("");
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

  async function createChannel(e: React.FormEvent) {
    e.preventDefault();
    if (!activeGuild || !channelName.trim()) return;
    setError(null);
    try {
      await api.createChannel(
        activeGuild.id,
        channelName.trim(),
        channelKind,
        channels.length,
        channelKind === "category" || !channelParent ? undefined : channelParent,
      );
      setChannelName("");
      setAddingChannel(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not create the channel");
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
      {settingsOpen && activeGuild && (
        <ServerSettings
          guild={activeGuild}
          channels={channels}
          onClose={() => setSettingsOpen(false)}
        />
      )}

      <nav className="guilds">
        {guilds.map((g) => (
          <button
            key={g.id}
            className={g.id === activeGuild?.id ? "guild active" : "guild"}
            title={g.name}
            onClick={() => onSelectGuild(g)}
          >
            <Avatar name={g.name} imageKey={g.icon_key} className="guild-image" />
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

        <div className="channels-head">
          <span className="channel-name">{activeGuild?.name ?? "No server"}</span>
          {activeGuild && (
            <>
              {(mayManageRoles || mayBan) && (
                <button
                  className="link add-channel"
                  title="Server settings"
                  onClick={() => setSettingsOpen(true)}
                >
                  ⚙
                </button>
              )}
              {mayManageChannels && (
                <button
                  className="link add-channel"
                  title="New channel"
                  onClick={() => setAddingChannel(!addingChannel)}
                >
                  +
                </button>
              )}
            </>
          )}
        </div>

        {addingChannel && activeGuild && mayManageChannels && (
          <form className="inline-form" onSubmit={createChannel}>
            <input
              placeholder="channel name"
              value={channelName}
              autoFocus
              onChange={(e) => setChannelName(e.target.value)}
            />
            <select
              aria-label="Channel kind"
              value={channelKind}
              onChange={(e) => setChannelKind(e.target.value as "text" | "voice" | "category")}
            >
              <option value="text">Text</option>
              <option value="voice">Voice</option>
              <option value="category">Category</option>
            </select>
            {channelKind !== "category" && categoriesIn(channels).length > 0 && (
              <select
                aria-label="Category"
                value={channelParent}
                onChange={(e) => setChannelParent(e.target.value)}
              >
                <option value="">No category</option>
                {categoriesIn(channels).map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            )}
            <button type="submit">Create</button>
          </form>
        )}

        {grouped.map((group) => (
          <div key={group.category?.id ?? "loose"} className="channel-group">
            {group.category && (
              <div className="category">{group.category.name.toUpperCase()}</div>
            )}
            {group.channels.map((c, at) => (
              <div key={c.id} className="channel-row">
                <button
                  className={c.id === activeChannel?.id ? "channel active" : "channel"}
                  onClick={() => onSelectChannel(c)}
                >
                  <span className="channel-name">
                    {c.kind === "voice" ? "🔊" : "#"} {c.name}
                  </span>
                  {unread[c.id] && c.id !== activeChannel?.id && (
                    <span className="unread" aria-label="unread messages" />
                  )}
                  {mayManageChannels && group.channels.length > 1 && (
                    <span className="channel-move">
                      <span
                        role="button"
                        tabIndex={0}
                        aria-label={`Move ${c.name} up`}
                        aria-disabled={at === 0}
                        onClick={(e) => {
                          e.stopPropagation();
                          if (at > 0) void move(c.id, at - 1);
                        }}
                        onKeyDown={(e) => {
                          if (e.key !== "Enter" && e.key !== " ") return;
                          e.stopPropagation();
                          e.preventDefault();
                          if (at > 0) void move(c.id, at - 1);
                        }}
                      >
                        ↑
                      </span>
                      <span
                        role="button"
                        tabIndex={0}
                        aria-label={`Move ${c.name} down`}
                        aria-disabled={at === group.channels.length - 1}
                        onClick={(e) => {
                          e.stopPropagation();
                          if (at < group.channels.length - 1) void move(c.id, at + 1);
                        }}
                        onKeyDown={(e) => {
                          if (e.key !== "Enter" && e.key !== " ") return;
                          e.stopPropagation();
                          e.preventDefault();
                          if (at < group.channels.length - 1) void move(c.id, at + 1);
                        }}
                      >
                        ↓
                      </span>
                    </span>
                  )}
                </button>
                {c.kind === "voice" && (people.inVoice[c.id] ?? []).length > 0 && (
                  <div className="channel-callers">
                    {(people.inVoice[c.id] ?? []).map((id) => (
                      <div key={id} className="channel-caller">
                        <Avatar name={people.names[id] ?? id.slice(0, 8)} imageKey={people.avatars[id]} className="avatar tiny" />
                        <span className="muted">{session.labelOf(id)}</span>
                        {people.mutedInVoice[id] && <span className="muted">muted</span>}
                        {people.deafenedInVoice[id] && <span className="muted">can't hear</span>}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        ))}

        <div className="join-box">
          {activeGuild && mayInvite && (
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
