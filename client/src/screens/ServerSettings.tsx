import { useCallback, useEffect, useState, type ReactNode } from "react";
import { api, type Ban, type GuildMember, type Invite } from "../api";
import { inviteLink } from "../invites";
import {
  ADMINISTRATOR,
  allows,
  BAN_MEMBERS,
  has,
  KICK_MEMBERS,
  MANAGE_GUILD,
  MANAGE_ROLES,
  PERMISSIONS,
  summarise,
  VIEW_CHANNEL,
} from "../permissions";
import type { Channel, Guild, Overwrite, Role } from "../types/events.gen";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { Avatar } from "../ui/Avatar";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { IconButton } from "../ui/IconButton";
import { Menu, MenuItem } from "../ui/Menu";
import { Sheet } from "../ui/Sheet";
import { SettingsPanel, type SettingsTab } from "../ui/SettingsPanel";
import { Toggle } from "../ui/Toggle";
import { TriState, type Stance } from "../ui/TriState";
import { PlacePicture } from "./PlacePicture";

type Tab = "overview" | "roles" | "people" | "access" | "bans" | "links";

type Load = "loading" | "ready" | "failed";

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
  status,
  onClose,
  onChanged,
}: {
  guild: Guild;
  channels: Channel[];
  permissions: number;
  iconURL: string | null;
  status: Record<string, string>;
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
  const [sheets, setSheets] = useState<Record<string, Overwrite[]>>({});
  const [pickedChannel, setPickedChannel] = useState<string | null>(null);
  const [pickedTarget, setPickedTarget] = useState<string | null>(null);
  const [hideUnseen, setHideUnseen] = useState(false);
  const [naming, setNaming] = useState<{ role: Role | null; name: string } | null>(null);
  const [pickedID, setPickedID] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [dropping, setDropping] = useState<Role | null>(null);
  const [saving, setSaving] = useState(false);
  const [load, setLoad] = useState<Load>("ready");
  const [trouble, setTrouble] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<{
    action: "kick" | "ban";
    member: GuildMember;
  } | null>(null);

  const ranked = [...roles].sort((a, b) => {
    if (a.is_default !== b.is_default) return a.is_default ? 1 : -1;
    return b.position - a.position;
  });

  const picked = ranked.find((role) => role.id === pickedID) ?? ranked[0];
  const rankable = ranked.filter((role) => !role.is_default);
  const rankOf = (role: Role) => rankable.findIndex((entry) => entry.id === role.id);

  const openable = channels.filter((channel) => channel.kind !== "category");
  const channelPick =
    openable.find((channel) => channel.id === pickedChannel) ?? openable[0];

  const listFor = (channelID: string) => sheets[channelID] ?? [];

  const carries = (channelID: string, roleID: string) =>
    listFor(channelID).some(
      (entry) => entry.target_id === roleID && (entry.allow !== 0 || entry.deny !== 0),
    );

  const shapedBy = (channelID: string) => {
    const touched = listFor(channelID).filter((e) => e.allow !== 0 || e.deny !== 0);
    if (touched.length === 0) return "Open to everybody the role allows";
    return `${touched.length} ${touched.length === 1 ? "role is" : "roles are"} shaped here`;
  };

  const everyone = roles.find((role) => role.is_default);
  const targets = channelPick
    ? [
        ...(everyone ? [everyone] : []),
        ...ranked.filter(
          (role) => !role.is_default && carries(channelPick.id, role.id),
        ),
        ...(pickedTarget &&
        !carries(channelPick.id, pickedTarget) &&
        !roles.find((r) => r.id === pickedTarget)?.is_default
          ? roles.filter((r) => r.id === pickedTarget)
          : []),
      ]
    : [];
  const targetPick =
    targets.find((role) => role.id === pickedTarget) ?? targets[0];
  const spare = channelPick
    ? ranked.filter(
        (role) => !role.is_default && !targets.some((t) => t.id === role.id),
      )
    : [];

  const shutTo = (channelID: string) =>
    everyone ? stanceOf(channelID, everyone.id, VIEW_CHANNEL) === "deny" : false;

  const letIn = (channelID: string, roleID: string) =>
    stanceOf(channelID, roleID, VIEW_CHANNEL) === "allow";

  async function setPrivate(channelID: string, shut: boolean) {
    if (!everyone) return;
    await setStance(channelID, everyone.id, VIEW_CHANNEL, shut ? "deny" : "inherit");
  }

  const unseen = targetPick
    ? openable.filter((channel) => !canSee(channel.id, targetPick))
    : [];
  const listed =
    hideUnseen && targetPick
      ? openable.filter((channel) => canSee(channel.id, targetPick))
      : openable;

  const bypasses = has(permissions, ADMINISTRATOR);

  function canSee(channelID: string, role: Role | undefined): boolean {
    if (!role) return true;
    if (has(role.permissions, ADMINISTRATOR)) return true;
    const stance = stanceOf(channelID, role.id, VIEW_CHANNEL);
    if (stance === "allow") return true;
    if (stance === "deny") return false;
    return has(role.permissions, VIEW_CHANNEL);
  }

  function effect(role: Role, channelID: string, bit: number, about: string): string {
    if (has(role.permissions, ADMINISTRATOR)) {
      return `${role.name} has Administrator, so this applies whatever is set here`;
    }
    const stance = stanceOf(channelID, role.id, bit);
    const base = has(role.permissions, bit);
    if (stance === "allow") {
      return base ? "Allowed here" : `Allowed here, though ${role.name} does not have it elsewhere`;
    }
    if (stance === "deny") {
      return base ? `Denied here, though ${role.name} has it elsewhere` : "Denied here";
    }
    return base
      ? `${role.name} has this, so it applies here`
      : `${role.name} does not have this${about ? "" : ""}`;
  }

  function stanceOf(channelID: string, roleID: string, bit: number): Stance {
    const entry = listFor(channelID).find((e) => e.target_id === roleID);
    if (!entry) return "inherit";
    if (has(entry.allow, bit)) return "allow";
    if (has(entry.deny, bit)) return "deny";
    return "inherit";
  }

  async function setStance(channelID: string, roleID: string, bit: number, next: Stance) {
    const held = listFor(channelID).find((e) => e.target_id === roleID);
    const was = {
      allow: held?.allow ?? 0,
      deny: held?.deny ?? 0,
    };
    const allow = next === "allow" ? was.allow | bit : was.allow & ~bit;
    const deny = next === "deny" ? was.deny | bit : was.deny & ~bit;

    const before = listFor(channelID);
    const after =
      allow === 0 && deny === 0
        ? before.filter((e) => e.target_id !== roleID)
        : before.some((e) => e.target_id === roleID)
          ? before.map((e) =>
              e.target_id === roleID ? { ...e, allow, deny } : e,
            )
          : [
              ...before,
              {
                channel_id: channelID,
                target_id: roleID,
                target_type: "role",
                allow,
                deny,
              },
            ];
    setSheets((held) => ({ ...held, [channelID]: after }));

    try {
      if (allow === 0 && deny === 0) await api.clearOverwrite(channelID, roleID);
      else await api.setOverwrite(channelID, roleID, allow, deny);
    } catch {
      setSheets((held) => ({ ...held, [channelID]: before }));
      setTrouble("That channel rule was not changed.");
    }
  }

  const wanted = filter.trim().toLowerCase();
  const shown = wanted
    ? PERMISSIONS.filter(
        (entry) =>
          entry.name.toLowerCase().includes(wanted) ||
          entry.about.toLowerCase().includes(wanted),
      )
    : PERMISSIONS;

  async function attempt(what: string, run: () => Promise<void>) {
    try {
      await run();
    } catch {
      setTrouble(what);
    }
  }

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
      setTrouble(`${entryName(bit)} was not changed for ${role.name}.`);
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
    await attempt("The order was not changed. It may be part way there — reopen to see.", () =>
      renumber(order),
    );
  }

  const canManage = allows(permissions, MANAGE_GUILD);
  const canRoles = allows(permissions, MANAGE_ROLES);

  const openTab = useCallback(async () => {
    if (tab === "overview") {
      setLoad("ready");
      return;
    }
    setLoad("loading");
    try {
      if (tab === "roles" || tab === "access") setRoles(await api.roles(guild.id));
      if (tab === "access") {
        const pairs = await Promise.all(
          channels
            .filter((channel) => channel.kind !== "category")
            .map(async (channel) => {
              const list = await api.overwrites(channel.id);
              return [channel.id, list] as [string, Overwrite[]];
            }),
        );
        const held: Record<string, Overwrite[]> = {};
        for (const [id, list] of pairs) {
          held[id] = list.filter((entry) => entry.target_type === "role");
        }
        setSheets(held);
      }
      if (tab === "people") setMembers(await api.members(guild.id));
      if (tab === "bans") setBans(await api.bans(guild.id));
      if (tab === "links") setInvites(await api.invites(guild.id));
      setLoad("ready");
    } catch {
      setLoad("failed");
    }
  }, [tab, guild.id, channels]);

  useEffect(() => {
    void openTab();
  }, [openTab]);

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
      {trouble && (
        <div className="settings-trouble" role="alert">
          <span>{trouble}</span>
          <button type="button" className="settings-trouble-go" onClick={() => setTrouble(null)}>
            Dismiss
          </button>
        </div>
      )}

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
                onClick={() =>
                  void attempt("The picture was not removed.", async () => {
                    await api.clearGuildIcon(guild.id);
                    onChanged();
                  })
                }
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
                await attempt("That name was not saved.", async () => {
                  await api.updateGuild(guild.id, { name: name.trim() });
                  onChanged();
                });
                setSaving(false);
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
          {ranked.length === 0 ? (
            load !== "ready" ? (
              <Waiting load={load} what="roles" onRetry={openTab} />
            ) : null
          ) : (
            <div className="roles">
              <div className="roles-list">
                {ranked.map((role) => (
                  <button
                    key={role.id}
                    type="button"
                    className="role-row"
                    data-active={picked?.id === role.id}
                    onClick={() => setPickedID(role.id)}
                  >
                    <span className="role-row-text">
                      <span className="role-row-name">{role.name}</span>
                      <span className="role-row-meta">
                        {role.is_default ? "Everybody" : summarise(role.permissions)}
                      </span>
                    </span>
                  </button>
                ))}
              </div>

              {picked && (
                <div className="role-detail">
                  <div className="role-detail-head">
                    <span className="role-detail-text">
                      <span className="role-detail-name">{picked.name}</span>
                      <span className="role-detail-meta">
                        {picked.is_default
                          ? "Everybody who joins has this, and it cannot be removed."
                          : `Rank ${rankOf(picked) + 1} of ${rankable.length}`}
                      </span>
                    </span>
                    {canRoles && !picked.is_default && (
                      <span className="role-detail-tools">
                        <IconButton
                          name="arrow-up"
                          size={13}
                          label={`Raise ${picked.name}`}
                          disabled={rankOf(picked) === 0}
                          onClick={() => void move(picked, -1)}
                        />
                        <IconButton
                          name="arrow-down"
                          size={13}
                          label={`Lower ${picked.name}`}
                          disabled={rankOf(picked) === rankable.length - 1}
                          onClick={() => void move(picked, 1)}
                        />
                        <IconButton
                          name="pencil-simple"
                          size={13}
                          label={`Rename ${picked.name}`}
                          onClick={() => setNaming({ role: picked, name: picked.name })}
                        />
                        <IconButton
                          name="trash"
                          size={13}
                          state="danger"
                          label={`Delete ${picked.name}`}
                          onClick={() => setDropping(picked)}
                        />
                      </span>
                    )}
                  </div>

                  <input
                    className="input role-filter"
                    value={filter}
                    placeholder="Search permissions"
                    onChange={(event) => setFilter(event.target.value)}
                  />

                  <div className="perm-list">
                    {shown.length === 0 ? (
                      <span className="perm-empty">Nothing matches that.</span>
                    ) : (
                      shown.map((entry) => {
                        const held = has(picked.permissions, entry.bit);
                        const implied =
                          !held &&
                          entry.bit !== ADMINISTRATOR &&
                          has(picked.permissions, ADMINISTRATOR);
                        return (
                          <div className="perm-row" key={entry.bit} data-implied={implied}>
                            <span className="perm-text">
                              <span className="perm-name">{entry.name}</span>
                              <span className="perm-about">
                                {implied ? "Administrator already grants this" : entry.about}
                              </span>
                            </span>
                            <Toggle
                              on={held}
                              label={`${entry.name} for ${picked.name}`}
                              disabled={!canRoles}
                              onChange={(on) => void setPermission(picked, entry.bit, on)}
                            />
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </>
      )}
      {tab === "people" && (
        <>
          <SettingsHead
            title="People"
            note={
              load === "ready"
                ? `${members.filter((m) => (status[m.user_id] ?? "offline") !== "offline").length} online, ${members.length} total`
                : undefined
            }
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
                  onClick={() => setConfirming({ action: "kick", member })}
                >
                  Kick
                </Button>
                <Button
                  kind="danger"
                  disabled={!allows(permissions, BAN_MEMBERS)}
                  onClick={() => setConfirming({ action: "ban", member })}
                >
                  Ban
                </Button>
              </div>
            ))}
            {load !== "ready" && <Waiting load={load} what="the member list" onRetry={openTab} />}
          </div>
        </>
      )}

      {tab === "access" && (
        <>
          <SettingsHead
            title="Channel access"
            note="What each role may do in one channel. Inherit leaves it to whatever the role already grants; an allow here beats a deny from another role."
          />
          {bypasses && (
            <p className="access-note">
              None of this applies to you. The owner and anybody holding Administrator keep
              every permission in every channel, so changing a rule here will not change what
              you can do — only what everybody else can.
            </p>
          )}
          {openable.length === 0 ? (
            load !== "ready" ? (
              <Waiting load={load} what="channel access" onRetry={openTab} />
            ) : (
              <span className="perm-empty">There are no channels yet.</span>
            )
          ) : (
            <div className="roles">
              <div className="channel-column">
                {targetPick && (
                  <div className="unseen-switch">
                    <span className="unseen-switch-text">
                      {unseen.length === 0
                        ? `${targetPick.name} can see every channel`
                        : `${targetPick.name} cannot see ${unseen.length} of ${openable.length}`}
                    </span>
                    <Toggle
                      on={hideUnseen}
                      label={`Hide what ${targetPick.name} cannot see`}
                      disabled={unseen.length === 0}
                      onChange={setHideUnseen}
                    />
                  </div>
                )}
                <div className="roles-list">
                  {listed.map((channel) => {
                    const seen = canSee(channel.id, targetPick);
                    return (
                      <button
                        key={channel.id}
                        type="button"
                        className="role-row"
                        data-active={channelPick?.id === channel.id}
                        data-unseen={targetPick ? !seen : undefined}
                        onClick={() => setPickedChannel(channel.id)}
                      >
                        <span className="channel-glyph">
                          <Icon
                            name={
                              targetPick && !seen
                                ? "eye-slash"
                                : channel.kind === "voice"
                                  ? "speaker-high"
                                  : "hash"
                            }
                            size={13}
                          />
                        </span>
                        <span className="role-row-text">
                          <span className="role-row-name">{channel.name}</span>
                          <span className="role-row-meta">
                            {targetPick && !seen ? "Hidden from this role" : shapedBy(channel.id)}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                  {listed.length === 0 && (
                    <span className="perm-empty">Every channel is hidden from this role.</span>
                  )}
                </div>
              </div>

              {channelPick && (
                <div className="role-detail">
                  <div className="role-detail-head">
                    <span className="role-detail-text">
                      <span className="role-detail-name">{channelPick.name}</span>
                      <span className="role-detail-meta">{shapedBy(channelPick.id)}</span>
                    </span>
                  </div>

                  <div className="visibility">
                    <div className="visibility-head">
                      <span className="visibility-text">
                        <span className="visibility-title">Private channel</span>
                        <span className="visibility-note">
                          {shutTo(channelPick.id)
                            ? "Only the roles ticked below can see it."
                            : "Anybody who can see the server can see this channel."}
                        </span>
                      </span>
                      <Toggle
                        on={shutTo(channelPick.id)}
                        label={`Make ${channelPick.name} private`}
                        disabled={!canRoles}
                        onChange={(shut) => void setPrivate(channelPick.id, shut)}
                      />
                    </div>
                    {shutTo(channelPick.id) && (
                      <div className="visibility-roles">
                        {ranked.filter((role) => !role.is_default).length === 0 ? (
                          <span className="visibility-empty">
                            There are no other roles yet, so nobody can see it.
                          </span>
                        ) : (
                          ranked
                            .filter((role) => !role.is_default)
                            .map((role) => {
                              const inside = letIn(channelPick.id, role.id);
                              return (
                                <button
                                  key={role.id}
                                  type="button"
                                  className="visibility-role"
                                  data-on={inside}
                                  disabled={!canRoles}
                                  onClick={() =>
                                    void setStance(
                                      channelPick.id,
                                      role.id,
                                      VIEW_CHANNEL,
                                      inside ? "inherit" : "allow",
                                    )
                                  }
                                >
                                  <Icon name={inside ? "check" : "plus"} size={12} />
                                  {role.name}
                                </button>
                              );
                            })
                        )}
                      </div>
                    )}
                  </div>

                  <div className="target-row">
                    {targets.map((role) => (
                      <button
                        key={role.id}
                        type="button"
                        className="target-chip"
                        data-active={targetPick?.id === role.id}
                        onClick={() => setPickedTarget(role.id)}
                      >
                        {role.name}
                        {carries(channelPick.id, role.id) && <span className="target-mark" />}
                      </button>
                    ))}
                    {canRoles && spare.length > 0 && (
                      <DropdownMenu.Root modal={false}>
                        <DropdownMenu.Trigger asChild>
                          <button type="button" className="target-chip target-add">
                            Add a role
                          </button>
                        </DropdownMenu.Trigger>
                        <Menu align="start">
                          {spare.map((role) => (
                            <MenuItem
                              key={role.id}
                              label={role.name}
                              onClick={() => setPickedTarget(role.id)}
                            />
                          ))}
                        </Menu>
                      </DropdownMenu.Root>
                    )}
                  </div>

                  {targetPick && (
                    <div className="perm-list">
                      {PERMISSIONS.map((entry) => {
                        const stance = stanceOf(channelPick.id, targetPick.id, entry.bit);
                        return (
                          <div className="perm-row" key={entry.bit}>
                            <span className="perm-text">
                              <span className="perm-name">{entry.name}</span>
                              <span className="perm-about" data-stance={stance}>
                                {effect(targetPick, channelPick.id, entry.bit, entry.about)}
                              </span>
                            </span>
                            <TriState
                              value={stance}
                              label={`${entry.name} for ${targetPick.name} in ${channelPick.name}`}
                              disabled={!canRoles}
                              onChange={(next) =>
                                void setStance(channelPick.id, targetPick.id, entry.bit, next)
                              }
                            />
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
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
                  onClick={() =>
                    void attempt(`${ban.username} was not unbanned.`, async () => {
                      await api.unban(guild.id, ban.user_id);
                      setBans(await api.bans(guild.id));
                    })
                  }
                >
                  Lift it
                </Button>
              </div>
            ))}
            {load === "ready" && bans.length === 0 && (
              <span className="settings-empty">Nobody is banned.</span>
            )}
            {load !== "ready" && <Waiting load={load} what="the ban list" onRetry={openTab} />}
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
                onClick={() =>
                  void attempt("A new link was not made.", async () => {
                    await api.createInvite(guild.id);
                    setInvites(await api.invites(guild.id));
                  })
                }
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
                  onClick={() =>
                    void navigator.clipboard
                      .writeText(inviteLink(invite.code))
                      .catch(() => setTrouble("That link was not copied."))
                  }
                >
                  Copy
                </Button>
                <Button
                  kind="danger"
                  onClick={() =>
                    void attempt("That link was not revoked.", async () => {
                      await api.revokeInvite(invite.code);
                      setInvites(await api.invites(guild.id));
                    })
                  }
                >
                  Revoke
                </Button>
              </div>
            ))}
            {load === "ready" && invites.length === 0 && (
              <span className="settings-empty">No links yet.</span>
            )}
            {load !== "ready" && <Waiting load={load} what="the links" onRetry={openTab} />}
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
              onClick={() => {
                const wanted = naming.name.trim();
                const role = naming.role;
                setNaming(null);
                void attempt(
                  role ? `${role.name} was not renamed.` : `${wanted} was not made.`,
                  async () => {
                    if (role) {
                      await api.updateRole(role.id, { name: wanted });
                      await refreshRoles();
                    } else {
                      const made = await api.createRole(guild.id, wanted, 0);
                      await renumber([...ranked.filter((r) => !r.is_default), made]);
                    }
                  },
                );
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
              onClick={() => {
                const role = dropping;
                setDropping(null);
                void attempt(`${role.name} was not deleted.`, async () => {
                  await api.deleteRole(role.id);
                  await refreshRoles();
                });
              }}
            >
              Delete it
            </Button>
          </div>
        </Sheet>
      )}

      {confirming && (
        <Sheet
          title={confirming.action === "kick" ? "Kick member" : "Ban member"}
          subtitle={
            confirming.action === "kick"
              ? `${confirming.member.username} can come back with a new invite. What they wrote stays.`
              : `${confirming.member.username} is kept out by account id only, so a new account gets back in. What they wrote stays.`
          }
          onClose={() => setConfirming(null)}
        >
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setConfirming(null)}>
              Never mind
            </Button>
            <Button
              kind="danger"
              onClick={() => {
                const { action, member } = confirming;
                setConfirming(null);
                void attempt(
                  action === "kick"
                    ? `${member.username} was not kicked.`
                    : `${member.username} was not banned.`,
                  async () => {
                    if (action === "kick") await api.kick(guild.id, member.user_id);
                    else await api.ban(guild.id, member.user_id);
                    setMembers(await api.members(guild.id));
                  },
                );
              }}
            >
              {confirming.action === "kick" ? "Kick them" : "Ban them"}
            </Button>
          </div>
        </Sheet>
      )}

      {placing && (
        <PlacePicture
          file={placing}
          onCancel={() => setPlacing(null)}
          onUse={(cropped) => {
            setPlacing(null);
            void attempt("That picture was not saved.", async () => {
              await api.setGuildIcon(guild.id, cropped);
              onChanged();
            });
          }}
        />
      )}
    </SettingsPanel>
  );
}

function Waiting({
  load,
  what,
  onRetry,
}: {
  load: Load;
  what: string;
  onRetry: () => void;
}) {
  if (load === "loading") {
    return <span className="settings-empty">Fetching {what}…</span>;
  }
  return (
    <div className="settings-failed" role="alert">
      <span>Could not fetch {what}. Nothing here is missing — this window could not read it.</span>
      <button type="button" className="settings-trouble-go" onClick={onRetry}>
        Try again
      </button>
    </div>
  );
}

function entryName(bit: number): string {
  return PERMISSIONS.find((entry) => entry.bit === bit)?.name ?? "That permission";
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
