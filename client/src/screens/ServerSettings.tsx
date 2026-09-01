import { useEffect, useState, type ReactNode } from "react";
import { api, type Ban, type GuildMember, type Invite } from "../api";
import { inviteLink } from "../invites";
import {
  allows,
  BAN_MEMBERS,
  KICK_MEMBERS,
  MANAGE_GUILD,
  MANAGE_ROLES,
  PERMISSIONS,
  VIEW_CHANNEL,
} from "../permissions";
import type { Channel, Guild, Overwrite, Role } from "../types/events.gen";
import { Avatar } from "../ui/Avatar";
import { Button } from "../ui/Button";
import { IconButton } from "../ui/IconButton";
import { Sheet } from "../ui/Sheet";
import { SettingsPanel, type SettingsTab } from "../ui/SettingsPanel";
import { Toggle } from "../ui/Toggle";
import { PlacePicture } from "./PlacePicture";

type Tab = "overview" | "roles" | "people" | "access" | "bans" | "links";

const TABS: SettingsTab<Tab>[] = [
  { id: "overview", label: "Overview", icon: "gear-six" },
  { id: "roles", label: "Roles", icon: "gear-six" },
  { id: "people", label: "People", icon: "gear-six" },
  { id: "access", label: "Channel access", icon: "gear-six" },
  { id: "bans", label: "Bans", icon: "gear-six" },
  { id: "links", label: "Links", icon: "paperclip" },
];

export function ServerSettings({
  guild,
  channels,
  permissions,
  iconURL,
  online,
  onClose,
  onChanged,
}: {
  guild: Guild;
  channels: Channel[];
  permissions: number;
  iconURL: string | null;
  online: Record<string, boolean>;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [tab, setTab] = useState<Tab>("overview");
  const [name, setName] = useState(guild.name);
  const [roles, setRoles] = useState<Role[]>([]);
  const [members, setMembers] = useState<GuildMember[]>([]);
  const [bans, setBans] = useState<Ban[]>([]);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [placing, setPlacing] = useState<File | null>(null);
  const [hidden, setHidden] = useState<Record<string, boolean>>({});
  const [naming, setNaming] = useState<{ role: Role | null; name: string } | null>(null);
  const [dropping, setDropping] = useState<Role | null>(null);
  const [saving, setSaving] = useState(false);

  const ranked = [...roles].sort((a, b) => {
    if (a.is_default !== b.is_default) return a.is_default ? 1 : -1;
    return b.position - a.position;
  });

  async function refreshRoles() {
    setRoles(await api.roles(guild.id));
  }

  async function setPermission(role: Role, bit: number, on: boolean) {
    const next = on ? role.permissions | bit : role.permissions & ~bit;
    setRoles((prev) => prev.map((r) => (r.id === role.id ? { ...r, permissions: next } : r)));
    try {
      await api.updateRole(role.id, { permissions: next });
    } catch {
      setRoles((prev) =>
        prev.map((r) => (r.id === role.id ? { ...r, permissions: role.permissions } : r)),
      );
    }
  }

  async function renumber(order: Role[]) {
    for (const [at, role] of order.entries()) {
      const wanted = order.length - at;
      if (role.position !== wanted) {
        await api.updateRole(role.id, { position: wanted });
      }
    }
    await refreshRoles();
  }

  async function move(role: Role, step: -1 | 1) {
    const order = ranked.filter((r) => !r.is_default);
    const at = order.findIndex((r) => r.id === role.id);
    if (at + step < 0 || at + step >= order.length) return;
    const [lifted] = order.splice(at, 1);
    order.splice(at + step, 0, lifted);
    await renumber(order);
  }

  const canManage = allows(permissions, MANAGE_GUILD);
  const canRoles = allows(permissions, MANAGE_ROLES);

  useEffect(() => {
    if (tab === "roles" || tab === "access") void api.roles(guild.id).then(setRoles);
    if (tab === "access") {
      void Promise.all(
        channels
          .filter((channel) => channel.kind !== "category")
          .map((channel) =>
            api
              .overwrites(channel.id)
              .then((list) => [channel.id, list] as [string, Overwrite[]])
              .catch(() => [channel.id, [] as Overwrite[]] as [string, Overwrite[]]),
          ),
      ).then((pairs) => {
        const shut: Record<string, boolean> = {};
        for (const [id, list] of pairs) {
          shut[id] = list.some(
            (o) => o.target_type === "role" && (o.deny & VIEW_CHANNEL) !== 0,
          );
        }
        setHidden(shut);
      });
    }
    if (tab === "people") void api.members(guild.id).then(setMembers);
    if (tab === "bans") void api.bans(guild.id).then(setBans);
    if (tab === "links") void api.invites(guild.id).then(setInvites);
  }, [tab, guild.id]);

  return (
    <SettingsPanel
      name={guild.name}
      kind="Settings"
      avatarURL={iconURL}
      tabs={TABS}
      active={tab}
      onPick={setTab}
      note={
        canRoles
          ? "You see this because you hold Manage roles. Nothing here is offered to a member who does not."
          : "You are looking at what you may change. Most of this needs Manage roles."
      }
      onClose={onClose}
    >
      {tab === "overview" && (
        <>
          <SettingsHead title="Overview" />
          <div className="settings-picture-row">
            <Avatar name={guild.name} url={iconURL} size={56} />
            <span className="settings-picture-text">
              Its icon, for now. A picture can be added afterwards, from this same panel.
            </span>
            <span className="settings-row-actions">
              <Button
                kind="quiet"
                disabled={!canManage}
                onClick={() => {
                  const picker = document.createElement("input");
                  picker.type = "file";
                  picker.accept = "image/*";
                  picker.onchange = () => {
                    const file = picker.files?.[0];
                    if (file) setPlacing(file);
                  };
                  picker.click();
                }}
              >
                Change picture
              </Button>
              <Button
                kind="quiet"
                disabled={!canManage || !guild.icon_key}
                onClick={async () => {
                  await api.clearGuildIcon(guild.id);
                  onChanged();
                }}
              >
                Remove
              </Button>
            </span>
          </div>

          <label className="field">
            <span className="field-label">Server name</span>
            <input
              className="input"
              value={name}
              disabled={!canManage}
              onChange={(event) => setName(event.target.value)}
            />
          </label>

          <label className="field">
            <span className="field-label">Description</span>
            <div className="needs-backend">
              A server carries only a name and a picture today. There is nowhere to keep a
              description, so this is not offered as a box that forgets what you type.
            </div>
          </label>

          <div className="settings-actions">
            <Button
              disabled={!canManage || saving || !name.trim() || name.trim() === guild.name}
              onClick={async () => {
                setSaving(true);
                try {
                  await api.updateGuild(guild.id, { name: name.trim() });
                  onChanged();
                } finally {
                  setSaving(false);
                }
              }}
            >
              {saving ? "Saving…" : "Save changes"}
            </Button>
          </div>
        </>
      )}

      {tab === "roles" && (
        <>
          <SettingsHead
            title="Roles"
            note="Highest first. A role can only be changed by somebody who outranks it."
            action={
              canRoles && (
                <Button onClick={() => setNaming({ role: null, name: "" })}>New role</Button>
              )
            }
          />
          <div className="matrix">
            <div className="matrix-row">
              <span className="matrix-cell matrix-head">Permission</span>
              {ranked.map((role, at) => (
                <span className="matrix-cell matrix-role" key={role.id}>
                  <span className="matrix-role-name">{role.name}</span>
                  {canRoles && !role.is_default && (
                    <span className="matrix-role-tools">
                      <IconButton
                        name="arrow-up"
                        size={13}
                        label={`Raise ${role.name}`}
                        disabled={at === 0}
                        onClick={() => void move(role, -1)}
                      />
                      <IconButton
                        name="arrow-down"
                        size={13}
                        label={`Lower ${role.name}`}
                        disabled={at === ranked.filter((r) => !r.is_default).length - 1}
                        onClick={() => void move(role, 1)}
                      />
                      <IconButton
                        name="pencil-simple"
                        size={13}
                        label={`Rename ${role.name}`}
                        onClick={() => setNaming({ role, name: role.name })}
                      />
                      <IconButton
                        name="trash"
                        size={13}
                        state="danger"
                        label={`Delete ${role.name}`}
                        onClick={() => setDropping(role)}
                      />
                    </span>
                  )}
                </span>
              ))}
            </div>
            {PERMISSIONS.map((entry) => (
              <div className="matrix-row" key={entry.bit}>
                <span className="matrix-cell matrix-head">{entry.name}</span>
                {ranked.map((role) => (
                  <span className="matrix-cell" key={role.id}>
                    <Toggle
                      on={allows(role.permissions, entry.bit)}
                      label={`${entry.name} for ${role.name}`}
                      disabled={!canRoles}
                      onChange={(on) => void setPermission(role, entry.bit, on)}
                    />
                  </span>
                ))}
              </div>
            ))}
            {roles.length === 0 && <span className="settings-empty">No roles yet.</span>}
          </div>
        </>
      )}

      {tab === "people" && (
        <>
          <SettingsHead
            title="People"
            note={`${members.filter((m) => online[m.user_id]).length} online, ${members.length} total`}
          />
          <div className="settings-list">
            {members.map((member) => (
              <div className="settings-list-row" key={member.user_id}>
                <Avatar name={member.username} size={28} />
                <span className="settings-list-name">{member.username}</span>
                <span className="settings-list-meta">#{member.discriminator}</span>
                <Button
                  kind="quiet"
                  disabled={!allows(permissions, KICK_MEMBERS)}
                  onClick={async () => {
                    await api.kick(guild.id, member.user_id);
                    setMembers(await api.members(guild.id));
                  }}
                >
                  Kick
                </Button>
                <Button
                  kind="danger"
                  disabled={!allows(permissions, BAN_MEMBERS)}
                  onClick={async () => {
                    await api.ban(guild.id, member.user_id);
                    setMembers(await api.members(guild.id));
                  }}
                >
                  Ban
                </Button>
              </div>
            ))}
          </div>
        </>
      )}

      {tab === "access" && (
        <>
          <SettingsHead
            title="Channel access"
            note="Toggle who can see each channel. Off means private, invite only."
          />
          <div className="access-list">
            {channels
              .filter((channel) => channel.kind !== "category")
              .map((channel) => (
                <div className="access-row" key={channel.id}>
                  <span className="access-name">{channel.name}</span>
                  <Toggle
                    on={!hidden[channel.id]}
                    label={`Everyone can see ${channel.name}`}
                    disabled={!canRoles}
                    onChange={async (on) => {
                      const everyone = roles.find((role) => role.is_default);
                      if (!everyone) return;
                      setHidden((was) => ({ ...was, [channel.id]: !on }));
                      try {
                        if (on) {
                          await api.clearOverwrite(channel.id, everyone.id);
                        } else {
                          await api.setOverwrite(channel.id, everyone.id, 0, VIEW_CHANNEL);
                        }
                      } catch {
                        setHidden((was) => ({ ...was, [channel.id]: on }));
                      }
                    }}
                  />
                </div>
              ))}
          </div>
        </>
      )}

      {tab === "bans" && (
        <>
          <SettingsHead
            title="Bans"
            note="People removed from this server. Unbanning lets them back in with a fresh invite."
          />
          <div className="settings-list">
            {bans.map((ban) => (
              <div className="settings-list-row" key={ban.user_id}>
                <Avatar name={ban.username} size={28} />
                <span className="settings-list-name">{ban.username}</span>
                {ban.reason && <span className="settings-list-meta">{ban.reason}</span>}
                <Button
                  kind="quiet"
                  disabled={!allows(permissions, BAN_MEMBERS)}
                  onClick={async () => {
                    await api.unban(guild.id, ban.user_id);
                    setBans(await api.bans(guild.id));
                  }}
                >
                  Lift it
                </Button>
              </div>
            ))}
            {bans.length === 0 && <span className="settings-empty">Nobody is banned.</span>}
          </div>
        </>
      )}

      {tab === "links" && (
        <>
          <SettingsHead
            title="Links"
            note={`Invite links people can use to join ${guild.name}.`}
            action={
              <Button
                onClick={async () => {
                  await api.createInvite(guild.id);
                  setInvites(await api.invites(guild.id));
                }}
              >
                New link
              </Button>
            }
          />
          <div className="settings-list">
            {invites.map((invite) => (
              <div className="settings-list-row" key={invite.code}>
                <span className="settings-list-name">{inviteLink(invite.code)}</span>
                <Button
                  kind="quiet"
                  onClick={() => void navigator.clipboard.writeText(inviteLink(invite.code))}
                >
                  Copy
                </Button>
                <Button
                  kind="danger"
                  onClick={async () => {
                    await api.revokeInvite(invite.code);
                    setInvites(await api.invites(guild.id));
                  }}
                >
                  Revoke
                </Button>
              </div>
            ))}
            {invites.length === 0 && <span className="settings-empty">No links yet.</span>}
          </div>
        </>
      )}
      {naming && (
        <Sheet
          title={naming.role ? "Rename role" : "New role"}
          subtitle={
            naming.role
              ? "Only the name changes. Its permissions and rank stay as they are."
              : "It starts with no permissions and sits at the bottom of the order."
          }
          onClose={() => setNaming(null)}
        >
          <label className="field">
            <span className="field-label">Name</span>
            <input
              className="input"
              value={naming.name}
              placeholder="Moderator"
              autoFocus
              onChange={(event) => setNaming({ ...naming, name: event.target.value })}
            />
          </label>
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setNaming(null)}>
              Never mind
            </Button>
            <Button
              disabled={!naming.name.trim()}
              onClick={async () => {
                const wanted = naming.name.trim();
                const role = naming.role;
                setNaming(null);
                if (role) {
                  await api.updateRole(role.id, { name: wanted });
                  await refreshRoles();
                } else {
                  const made = await api.createRole(guild.id, wanted, 0);
                  await renumber([...ranked.filter((r) => !r.is_default), made]);
                }
              }}
            >
              {naming.role ? "Rename it" : "Make it"}
            </Button>
          </div>
        </Sheet>
      )}

      {dropping && (
        <Sheet
          title={`Delete ${dropping.name}`}
          subtitle="Everybody who holds it loses whatever it granted them. This cannot be undone."
          onClose={() => setDropping(null)}
        >
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setDropping(null)}>
              Never mind
            </Button>
            <Button
              kind="danger"
              onClick={async () => {
                const role = dropping;
                setDropping(null);
                await api.deleteRole(role.id);
                await refreshRoles();
              }}
            >
              Delete it
            </Button>
          </div>
        </Sheet>
      )}

      {placing && (
        <PlacePicture
          file={placing}
          onCancel={() => setPlacing(null)}
          onUse={async (cropped) => {
            setPlacing(null);
            await api.setGuildIcon(guild.id, cropped);
            onChanged();
          }}
        />
      )}
    </SettingsPanel>
  );
}

function SettingsHead({
  title,
  note,
  action,
}: {
  title: string;
  note?: string;
  action?: ReactNode;
}) {
  return (
    <div className="settings-head" data-note={Boolean(note)}>
      <div className="settings-head-text">
        <span className="settings-title">{title}</span>
        {note && <span className="settings-note">{note}</span>}
      </div>
      {action}
    </div>
  );
}
