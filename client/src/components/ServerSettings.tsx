import { useCallback, useEffect, useState } from "react";
import { api, type Ban, type GuildMember } from "../api";
import {
  ADMINISTRATOR,
  BAN_MEMBERS,
  MANAGE_GUILD,
  MANAGE_ROLES,
  PERMISSIONS,
  allows,
  has,
  summarise,
  withBit,
} from "../permissions";
import { emptySession, session, type SessionState } from "../session";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";
import { PickImage } from "./PickImage";
import type { Channel, Guild, Overwrite, Role } from "../types/events.gen";

type Tab = "overview" | "roles" | "members" | "channels" | "bans";
type Cell = "allow" | "deny" | "inherit";

const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: "overview", label: "Overview", icon: "identification-card" },
  { id: "roles", label: "Roles", icon: "users-three" },
  { id: "members", label: "People", icon: "user" },
  { id: "channels", label: "Channel access", icon: "hash" },
  { id: "bans", label: "Bans", icon: "prohibit" },
];

function stateOf(overwrite: Overwrite | undefined, bit: number): Cell {
  if (!overwrite) return "inherit";
  if (has(overwrite.allow, bit)) return "allow";
  if (has(overwrite.deny, bit)) return "deny";
  return "inherit";
}

function nextState(current: Cell): Cell {
  if (current === "inherit") return "allow";
  if (current === "allow") return "deny";
  return "inherit";
}

function glyph(state: Cell): string {
  if (state === "allow") return "✓";
  if (state === "deny") return "✕";
  return "–";
}

export function ServerSettings({
  guild,
  channels,
  onClose,
}: {
  guild: Guild;
  channels: Channel[];
  onClose: () => void;
}) {
  const [chosen, setChosen] = useState<Tab | null>(null);
  const [icon, setIcon] = useState<string | null>(guild.icon_key ?? null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [members, setMembers] = useState<GuildMember[]>([]);
  const [error, setError] = useState<string | null>(null);

  const [selectedRole, setSelectedRole] = useState<string | null>(null);
  const [newRole, setNewRole] = useState("");
  const [nameDraft, setNameDraft] = useState("");

  const [selectedMember, setSelectedMember] = useState<string | null>(null);
  const [memberRoles, setMemberRoles] = useState<string[]>([]);

  const [selectedChannel, setSelectedChannel] = useState<string | null>(null);
  const [overwrites, setOverwrites] = useState<Overwrite[]>([]);

  const [bans, setBans] = useState<Ban[]>([]);
  const [lifting, setLifting] = useState<string | null>(null);
  const [people, setPeople] = useState<SessionState>(emptySession);

  useEffect(() => session.onChange(setPeople), []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const held = people.guildAllows[guild.id];
  const mayManageRoles = allows(held, MANAGE_ROLES);
  const mayBan = allows(held, BAN_MEMBERS);
  const mayManageGuild = allows(held, MANAGE_GUILD);

  const offered: Tab[] = [
    ...(mayManageGuild ? (["overview"] as Tab[]) : []),
    ...(mayManageRoles ? (["roles", "members", "channels"] as Tab[]) : []),
    ...(mayBan ? (["bans"] as Tab[]) : []),
  ];
  const tab = chosen && offered.includes(chosen) ? chosen : (offered[0] ?? null);

  const complain = (err: unknown, fallback: string) =>
    setError(err instanceof Error ? err.message : fallback);

  const loadRoles = useCallback(async () => {
    try {
      const list = await api.roles(guild.id);
      setRoles(list);
      setSelectedRole(
        (current) => current ?? list.find((r) => !r.is_default)?.id ?? list[0]?.id ?? null,
      );
    } catch (err) {
      complain(err, "could not load roles");
    }
  }, [guild.id]);

  useEffect(() => {
    if (!mayManageRoles) return;
    void loadRoles();
  }, [loadRoles, mayManageRoles]);

  useEffect(() => {
    if (tab !== "members") return;
    api.members(guild.id).then(setMembers).catch((err) => complain(err, "could not load people"));
  }, [tab, guild.id]);

  useEffect(() => {
    if (!selectedMember) return;
    api
      .memberRoles(guild.id, selectedMember)
      .then((list) => setMemberRoles(list.map((r) => r.id)))
      .catch((err) => complain(err, "could not load their roles"));
  }, [guild.id, selectedMember]);

  useEffect(() => {
    if (!selectedChannel) return;
    api
      .overwrites(selectedChannel)
      .then(setOverwrites)
      .catch((err) => complain(err, "could not load the overrides"));
  }, [selectedChannel]);

  const loadBans = useCallback(async () => {
    try {
      setBans(await api.bans(guild.id));
    } catch (err) {
      complain(err, "could not load the bans");
    }
  }, [guild.id]);

  useEffect(() => {
    if (tab !== "bans") return;
    void loadBans();
  }, [tab, loadBans]);

  const role = roles.find((r) => r.id === selectedRole) ?? null;

  useEffect(() => {
    setNameDraft(role?.name ?? "");
  }, [role?.id, role?.name]);

  async function createRole(e: React.FormEvent) {
    e.preventDefault();
    if (!newRole.trim()) return;
    setError(null);
    try {
      const created = await api.createRole(guild.id, newRole.trim(), 0);
      setNewRole("");
      setSelectedRole(created.id);
      await loadRoles();
    } catch (err) {
      complain(err, "could not create the role");
    }
  }

  async function togglePermission(bit: number, on: boolean) {
    if (!role) return;
    setError(null);
    const next = withBit(role.permissions, bit, on);
    setRoles((prev) => prev.map((r) => (r.id === role.id ? { ...r, permissions: next } : r)));
    try {
      await api.updateRole(role.id, { permissions: next });
    } catch (err) {
      complain(err, "could not change the permission");
      await loadRoles();
    }
  }

  async function renameRole() {
    const name = nameDraft.trim();
    if (!role || !name || name === role.name) return;
    setError(null);
    try {
      const updated = await api.updateRole(role.id, { name });
      setRoles((prev) => prev.map((r) => (r.id === role.id ? updated : r)));
    } catch (err) {
      complain(err, "could not rename the role");
      setNameDraft(role.name);
    }
  }

  async function moveRole(direction: -1 | 1) {
    if (!role) return;
    setError(null);
    try {
      await api.updateRole(role.id, { position: role.position + direction });
      await loadRoles();
    } catch (err) {
      complain(err, "could not move the role");
    }
  }

  async function removeRole() {
    if (!role) return;
    setError(null);
    try {
      await api.deleteRole(role.id);
      setSelectedRole(null);
      await loadRoles();
    } catch (err) {
      complain(err, "could not delete the role");
    }
  }

  async function toggleMemberRole(roleID: string, on: boolean) {
    if (!selectedMember) return;
    setError(null);
    try {
      if (on) await api.assignRole(guild.id, selectedMember, roleID);
      else await api.unassignRole(guild.id, selectedMember, roleID);
      setMemberRoles((prev) => (on ? [...prev, roleID] : prev.filter((id) => id !== roleID)));
    } catch (err) {
      complain(err, "could not change their roles");
    }
  }

  async function liftBan(userID: string) {
    setLifting(userID);
    setError(null);
    try {
      await api.unban(guild.id, userID);
      setBans((prev) => prev.filter((b) => b.user_id !== userID));
    } catch (err) {
      complain(err, "could not lift the ban");
      await loadBans();
    } finally {
      setLifting(null);
    }
  }

  async function cycle(roleID: string, bit: number, from: Cell) {
    if (!selectedChannel) return;
    setError(null);
    const next = nextState(from);

    const current = overwrites.find((o) => o.target_id === roleID);
    const allow = withBit(current?.allow ?? 0, bit, next === "allow");
    const deny = withBit(current?.deny ?? 0, bit, next === "deny");

    try {
      if (allow === 0 && deny === 0) await api.clearOverwrite(selectedChannel, roleID);
      else await api.setOverwrite(selectedChannel, roleID, allow, deny);
      setOverwrites(await api.overwrites(selectedChannel));
    } catch (err) {
      complain(err, "could not change the override");
    }
  }

  const editing = channels.find((c) => c.id === selectedChannel) ?? null;

  return (
    <div className="dialog-backdrop" onMouseDown={onClose}>
      <div
        className="settings-window"
        role="dialog"
        aria-modal="true"
        aria-label={`${guild.name} settings`}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <aside className="card settings-nav">
          <div className="settings-nav-head">
            <Avatar name={guild.name} imageKey={icon} mine />
            <div className="grow">
              <div className="identity-name clip">{guild.name}</div>
              <div className="identity-state">Settings</div>
            </div>
            <button className="icon-button" title="Close" onClick={onClose}>
              <Icon name="x" size={15} />
            </button>
          </div>

          <div className="settings-nav-list">
            {TABS.filter((t) => offered.includes(t.id)).map((t) => (
              <button
                key={t.id}
                className={t.id === tab ? "role-pick active" : "role-pick"}
                onClick={() => setChosen(t.id)}
              >
                <Icon name={t.icon} size={16} />
                {t.label}
              </button>
            ))}

            <div className="settings-nav-note">
              You see this because of what you hold here. Nothing on this list is offered to
              somebody who does not.
            </div>
          </div>
        </aside>

        <section className="card settings-main">
          <div className="settings-head">
            <div>
              <div className="settings-head-title">
                {tab === "channels" && editing ? (
                  <>
                    Editing <em>#{editing.name}</em>
                  </>
                ) : (
                  (TABS.find((t) => t.id === tab)?.label ?? "Settings")
                )}
              </div>
              <div className="settings-head-about">
                {tab === "channels"
                  ? "A cell overrides the role's own setting, here only. Deny wins over allow."
                  : tab === "roles"
                    ? "Position is what a role outranks. The everyone role cannot move out of last place."
                    : tab === "bans"
                      ? "Nobody banned is the normal state. Without this list a ban is permanent by accident."
                      : tab === "members"
                        ? "A person's roles decide what they can do, everywhere in the server."
                        : "The name and the picture the server is known by."}
              </div>
            </div>

            {tab === "channels" && (
              <div className="legend">
                <span>
                  <span className="cell allow">✓</span>Allowed here
                </span>
                <span>
                  <span className="cell deny">✕</span>Denied here
                </span>
                <span>
                  <span className="cell">–</span>Whatever the role says
                </span>
              </div>
            )}
          </div>

          <div className="settings-content">
            {error && (
              <div className="banner bad">
                <Icon name="warning-circle" size={15} />
                <span className="grow">{error}</span>
                <span className="banner-actions">
                  <button className="link quiet" onClick={() => setError(null)}>
                    Dismiss
                  </button>
                </span>
              </div>
            )}

            {tab === "overview" && (
              <PickImage
                name={guild.name}
                imageKey={icon}
                label="a picture"
                className="big"
                mine
                onChosen={setIcon}
                upload={async (file, onProgress) =>
                  (await api.setGuildIcon(guild.id, file, onProgress)).icon_key
                }
                remove={() => api.clearGuildIcon(guild.id)}
              />
            )}

            {tab === "roles" && (
              <div className="row top">
                <div className="settings-side">
                  {roles.map((r) => (
                    <button
                      key={r.id}
                      className={r.id === selectedRole ? "role-pick active" : "role-pick"}
                      onClick={() => setSelectedRole(r.id)}
                    >
                      <span className={r.is_default ? "matrix-swatch plain" : "matrix-swatch"} />
                      <span className="grow clip">{r.name}</span>
                      {r.is_default && <Icon name="lock-simple" size={12} />}
                    </button>
                  ))}

                  <form className="row" onSubmit={createRole}>
                    <input
                      className="input grow"
                      placeholder="new role"
                      value={newRole}
                      onChange={(e) => setNewRole(e.target.value)}
                    />
                    <button className="btn btn-primary btn-small" type="submit">
                      Add
                    </button>
                  </form>
                </div>

                <div className="grow stack">
                  {!role && <div className="note">Pick a role.</div>}
                  {role && (
                    <>
                      <div className="row">
                        <input
                          className="input grow"
                          value={nameDraft}
                          aria-label="Role name"
                          disabled={role.is_default}
                          onChange={(e) => setNameDraft(e.target.value)}
                          onBlur={() => void renameRole()}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") void renameRole();
                            if (e.key === "Escape") setNameDraft(role.name);
                          }}
                        />
                        <button
                          className="icon-button"
                          title="Move up"
                          disabled={role.is_default}
                          onClick={() => void moveRole(1)}
                        >
                          <Icon name="arrow-up" size={15} />
                        </button>
                        <button
                          className="icon-button"
                          title="Move down"
                          disabled={role.is_default}
                          onClick={() => void moveRole(-1)}
                        >
                          <Icon name="arrow-down" size={15} />
                        </button>
                        <button
                          className="icon-button danger"
                          title="Delete this role"
                          disabled={role.is_default}
                          onClick={() => void removeRole()}
                        >
                          <Icon name="trash" size={15} />
                        </button>
                      </div>

                      {role.is_default && (
                        <div className="note">
                          Everyone has this role. It cannot be renamed, moved or removed.
                        </div>
                      )}

                      {has(role.permissions, ADMINISTRATOR) && (
                        <div className="banner">
                          <Icon name="warning-circle" size={15} />
                          <span>
                            Administrator grants everything below, and anything added in future.
                          </span>
                        </div>
                      )}

                      <div className="stack tight">
                        {PERMISSIONS.map((permission) => (
                          <label key={permission.bit} className="check">
                            <input
                              type="checkbox"
                              checked={has(role.permissions, permission.bit)}
                              onChange={(e) =>
                                void togglePermission(permission.bit, e.target.checked)
                              }
                            />
                            <span>
                              {permission.name}
                              <span className="check-about"> — {permission.about}</span>
                            </span>
                          </label>
                        ))}
                      </div>
                    </>
                  )}
                </div>
              </div>
            )}

            {tab === "members" && (
              <div className="row top">
                <div className="settings-side">
                  {members.map((m) => (
                    <button
                      key={m.user_id}
                      className={m.user_id === selectedMember ? "role-pick active" : "role-pick"}
                      onClick={() => setSelectedMember(m.user_id)}
                    >
                      <Avatar name={m.username} imageKey={m.avatar_key} />
                      <span className="grow clip">{m.nickname ?? m.username}</span>
                      <span className="member-tag">#{m.discriminator}</span>
                    </button>
                  ))}
                </div>

                <div className="grow stack tight">
                  {!selectedMember && <div className="note">Pick somebody.</div>}
                  {selectedMember &&
                    roles.map((r) => (
                      <label key={r.id} className="check">
                        <input
                          type="checkbox"
                          checked={r.is_default || memberRoles.includes(r.id)}
                          disabled={r.is_default}
                          onChange={(e) => void toggleMemberRole(r.id, e.target.checked)}
                        />
                        <span>
                          {r.name}
                          <span className="check-about"> — {summarise(r.permissions)}</span>
                        </span>
                      </label>
                    ))}
                </div>
              </div>
            )}

            {tab === "channels" && (
              <>
                <div className="row wrap">
                  {channels
                    .filter((c) => c.kind !== "category")
                    .map((c) => (
                      <button
                        key={c.id}
                        className={c.id === selectedChannel ? "role-pick active" : "role-pick"}
                        onClick={() => setSelectedChannel(c.id)}
                      >
                        <Icon name={c.kind === "voice" ? "speaker-high" : "hash"} size={14} />
                        {c.name}
                      </button>
                    ))}
                </div>

                {!selectedChannel && <div className="note">Pick a channel.</div>}

                {selectedChannel && (
                  <div className="matrix">
                    <div className="matrix-head">
                      <div className="matrix-role kicker">Role</div>
                      {PERMISSIONS.map((p) => (
                        <div key={p.bit} className="matrix-column">
                          {p.name}
                        </div>
                      ))}
                    </div>

                    {roles.map((r) => {
                      const overwrite = overwrites.find((o) => o.target_id === r.id);
                      return (
                        <div key={r.id} className="matrix-row">
                          <div className="matrix-role">
                            <div className="matrix-role-name">
                              <span
                                className={r.is_default ? "matrix-swatch plain" : "matrix-swatch"}
                              />
                              <span className="clip">{r.name}</span>
                            </div>
                            <div className="matrix-role-sub">{summarise(r.permissions)}</div>
                          </div>

                          {PERMISSIONS.map((p) => {
                            const state = stateOf(overwrite, p.bit);
                            return (
                              <div key={p.bit} className="matrix-cell">
                                <button
                                  className={state === "inherit" ? "cell" : `cell ${state}`}
                                  aria-label={`${p.name} for ${r.name}: ${state}`}
                                  onClick={() => void cycle(r.id, p.bit, state)}
                                >
                                  {glyph(state)}
                                </button>
                              </div>
                            );
                          })}
                        </div>
                      );
                    })}
                  </div>
                )}

                {selectedChannel && (
                  <div className="dashed">
                    <div className="note">
                      A cell cycles: whatever the role says, allowed, denied. An override can also
                      be aimed at one person; the server takes it, nothing here offers it yet.
                    </div>
                    <span className="pending">per-person overrides · design only</span>
                  </div>
                )}
              </>
            )}

            {tab === "bans" && (
              <div className="stack tight">
                {bans.length === 0 && (
                  <div className="note">Nobody is banned from this server.</div>
                )}
                {bans.map((b) => (
                  <div
                    key={b.user_id}
                    className={lifting === b.user_id ? "list-row fading" : "list-row"}
                  >
                    <Avatar name={b.username} imageKey={null} />
                    <div className="grow">
                      <div className="clip">{b.username}</div>
                      <div className="field-note">
                        {new Date(b.created_at).toLocaleDateString()}
                        {b.banned_by && ` · by ${session.labelOf(b.banned_by)}`}
                        {b.reason ? ` · ${b.reason}` : ""}
                      </div>
                    </div>
                    <button
                      className="link"
                      disabled={lifting === b.user_id}
                      onClick={() => void liftBan(b.user_id)}
                    >
                      {lifting === b.user_id ? "lifting…" : "Lift"}
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
