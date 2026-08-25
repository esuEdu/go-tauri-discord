import { useCallback, useEffect, useState } from "react";
import { api, type Ban, type GuildMember } from "../api";
import { ADMINISTRATOR, BAN_MEMBERS, MANAGE_GUILD, MANAGE_ROLES, PERMISSIONS, allows, has, summarise, withBit } from "../permissions";
import { emptySession, session, type SessionState } from "../session";
import { PickImage } from "./PickImage";
import type { Channel, Guild, Overwrite, Role } from "../types/events.gen";

type Tab = "roles" | "members" | "channels" | "bans";

const TAB_NAMES: Record<Tab, string> = {
  roles: "Roles",
  members: "Members",
  channels: "Channel access",
  bans: "Bans",
};

type OverwriteState = "allow" | "deny" | "inherit";

function stateOf(overwrite: Overwrite | undefined, bit: number): OverwriteState {
  if (!overwrite) return "inherit";
  if (has(overwrite.allow, bit)) return "allow";
  if (has(overwrite.deny, bit)) return "deny";
  return "inherit";
}

export function ServerSettings({ guild, channels, onClose }: {
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
  const [people, setPeople] = useState<SessionState>(emptySession);

  useEffect(() => session.onChange(setPeople), []);

  const held = people.guildAllows[guild.id];
  const mayManageRoles = allows(held, MANAGE_ROLES);
  const mayBan = allows(held, BAN_MEMBERS);
  const mayManageGuild = allows(held, MANAGE_GUILD);

  const offered: Tab[] = [
    ...(mayManageRoles ? (["roles", "members", "channels"] as Tab[]) : []),
    ...(mayBan ? (["bans"] as Tab[]) : []),
  ];
  const tab = chosen && offered.includes(chosen) ? chosen : (offered[0] ?? null);
  const setTab = setChosen;

  const complain = (err: unknown, fallback: string) =>
    setError(err instanceof Error ? err.message : fallback);

  const loadRoles = useCallback(async () => {
    try {
      const list = await api.roles(guild.id);
      setRoles(list);
      setSelectedRole((current) => current ?? list.find((r) => !r.is_default)?.id ?? list[0]?.id ?? null);
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
    api.members(guild.id).then(setMembers).catch((err) => complain(err, "could not load members"));
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
      .catch((err) => complain(err, "could not load overwrites"));
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
    setError(null);
    try {
      await api.unban(guild.id, userID);
      setBans((prev) => prev.filter((b) => b.user_id !== userID));
    } catch (err) {
      complain(err, "could not lift the ban");
      await loadBans();
    }
  }

  async function setOverwrite(roleID: string, bit: number, next: OverwriteState) {
    if (!selectedChannel) return;
    setError(null);

    const current = overwrites.find((o) => o.target_id === roleID);
    const allow = withBit(current?.allow ?? 0, bit, next === "allow");
    const deny = withBit(current?.deny ?? 0, bit, next === "deny");

    try {
      if (allow === 0 && deny === 0) {
        await api.clearOverwrite(selectedChannel, roleID);
      } else {
        await api.setOverwrite(selectedChannel, roleID, allow, deny);
      }
      setOverwrites(await api.overwrites(selectedChannel));
    } catch (err) {
      complain(err, "could not change the overwrite");
    }
  }

  return (
    <div className="dialog-backdrop" onMouseDown={onClose}>
      <div
        className="dialog settings"
        role="dialog"
        aria-modal="true"
        aria-label={`${guild.name} settings`}
        onMouseDown={(e) => e.stopPropagation()}
      >
        {mayManageGuild && (
          <PickImage
            name={guild.name}
            imageKey={icon}
            label="server icon"
            onChosen={setIcon}
            upload={async (file) => (await api.setGuildIcon(guild.id, file)).icon_key}
            remove={() => api.clearGuildIcon(guild.id)}
          />
        )}

        <div className="settings-tabs">
          {offered.map((name) => (
            <button
              key={name}
              className={tab === name ? "link active" : "link"}
              onClick={() => setTab(name)}
            >
              {TAB_NAMES[name]}
            </button>
          ))}
          <span className="spacer" />
          <button className="secondary" onClick={onClose}>
            Done
          </button>
        </div>

        {error && <div className="error inline">{error}</div>}

        {tab === "roles" && (
          <div className="settings-body">
            <div className="settings-list">
              {roles.map((r) => (
                <button
                  key={r.id}
                  className={r.id === selectedRole ? "channel active" : "channel"}
                  onClick={() => setSelectedRole(r.id)}
                >
                  <span className="channel-name">{r.name}</span>
                  <span className="muted role-summary">{summarise(r.permissions)}</span>
                </button>
              ))}
              <form className="inline-form" onSubmit={createRole}>
                <input
                  placeholder="new role"
                  value={newRole}
                  onChange={(e) => setNewRole(e.target.value)}
                />
                <button type="submit">Add</button>
              </form>
            </div>

            <div className="settings-detail">
              {!role && <div className="muted">Pick a role.</div>}
              {role && (
                <>
                  <div className="role-head">
                    <input
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
                    <button className="link" onClick={() => void moveRole(1)} disabled={role.is_default}>
                      ↑
                    </button>
                    <button className="link" onClick={() => void moveRole(-1)} disabled={role.is_default}>
                      ↓
                    </button>
                    <button className="link danger" onClick={() => void removeRole()} disabled={role.is_default}>
                      Delete
                    </button>
                  </div>

                  {role.is_default && (
                    <div className="muted">
                      Everyone has this role. It cannot be renamed, moved or removed.
                    </div>
                  )}

                  {has(role.permissions, ADMINISTRATOR) && (
                    <div className="muted">
                      Administrator grants everything below, and anything added in future.
                    </div>
                  )}

                  <div className="permissions">
                    {PERMISSIONS.map((permission) => (
                      <label key={permission.bit} className="permission">
                        <input
                          type="checkbox"
                          checked={has(role.permissions, permission.bit)}
                          onChange={(e) => void togglePermission(permission.bit, e.target.checked)}
                        />
                        <span>
                          {permission.name}
                          <span className="muted permission-about"> — {permission.about}</span>
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
          <div className="settings-body">
            <div className="settings-list">
              {members.map((m) => (
                <button
                  key={m.user_id}
                  className={m.user_id === selectedMember ? "channel active" : "channel"}
                  onClick={() => setSelectedMember(m.user_id)}
                >
                  <span className="channel-name">{m.nickname ?? m.username}</span>
                </button>
              ))}
            </div>

            <div className="settings-detail">
              {!selectedMember && <div className="muted">Pick somebody.</div>}
              {selectedMember && (
                <div className="permissions">
                  {roles.map((r) => (
                    <label key={r.id} className="permission">
                      <input
                        type="checkbox"
                        checked={r.is_default || memberRoles.includes(r.id)}
                        disabled={r.is_default}
                        onChange={(e) => void toggleMemberRole(r.id, e.target.checked)}
                      />
                      <span>
                        {r.name}
                        <span className="muted permission-about"> — {summarise(r.permissions)}</span>
                      </span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {tab === "bans" && (
          <div className="settings-detail settings-single">
            {bans.length === 0 && <div className="muted">Nobody is banned from this server.</div>}
            {bans.map((b) => (
              <div key={b.user_id} className="ban">
                <div className="ban-who">
                  <span className="channel-name">{b.username}</span>
                  <span className="muted">
                    {new Date(b.created_at).toLocaleDateString()}
                    {b.banned_by && ` — by ${session.nameOf(b.banned_by)}`}
                  </span>
                </div>
                <span className="muted ban-reason">{b.reason ?? "No reason given."}</span>
                <button className="link" onClick={() => void liftBan(b.user_id)}>
                  Lift
                </button>
              </div>
            ))}
          </div>
        )}

        {tab === "channels" && (
          <div className="settings-body">
            <div className="settings-list">
              {channels
                .filter((c) => c.kind !== "category")
                .map((c) => (
                  <button
                    key={c.id}
                    className={c.id === selectedChannel ? "channel active" : "channel"}
                    onClick={() => setSelectedChannel(c.id)}
                  >
                    <span className="channel-name">
                      {c.kind === "voice" ? "🔊" : "#"} {c.name}
                    </span>
                  </button>
                ))}
            </div>

            <div className="settings-detail">
              {!selectedChannel && <div className="muted">Pick a channel.</div>}
              {selectedChannel && (
                <>
                  <div className="muted">
                    Inherit means the role's own permission decides. Deny always wins over allow.
                  </div>
                  {roles.map((r) => {
                    const overwrite = overwrites.find((o) => o.target_id === r.id);
                    return (
                      <div key={r.id} className="overwrite-role">
                        <div className="overwrite-name">{r.name}</div>
                        {PERMISSIONS.map((permission) => (
                          <div key={permission.bit} className="overwrite-row">
                            <span className="channel-name">{permission.name}</span>
                            {(["allow", "inherit", "deny"] as OverwriteState[]).map((choice) => (
                              <button
                                key={choice}
                                className={
                                  stateOf(overwrite, permission.bit) === choice
                                    ? `overwrite ${choice} on`
                                    : "overwrite"
                                }
                                onClick={() => void setOverwrite(r.id, permission.bit, choice)}
                              >
                                {choice === "allow" ? "✓" : choice === "deny" ? "✕" : "–"}
                              </button>
                            ))}
                          </div>
                        ))}
                      </div>
                    );
                  })}
                </>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
