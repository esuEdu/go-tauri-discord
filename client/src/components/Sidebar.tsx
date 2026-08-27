import { useCallback, useEffect, useState } from "react";
import { api } from "../api";
import { useDismiss } from "../dismiss";
import { emptySession, session, type SessionState } from "../session";
import { voice, type ScreenState } from "../voice";
import {
  CREATE_INVITE,
  MANAGE_CHANNELS,
  MANAGE_GUILD,
  MANAGE_ROLES,
  BAN_MEMBERS,
  allows,
} from "../permissions";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";
import { NewChannel } from "./NewChannel";
import { NewServer } from "./NewServer";
import { Invites } from "./Invites";
import { ServerSettings } from "./ServerSettings";
import { PersonMenu } from "./PersonMenu";
import type { Channel, Guild } from "../types/events.gen";

interface Props {
  guilds: Guild[];
  channels: Channel[];
  activeGuild: Guild | null;
  activeChannel: Channel | null;
  selfID: string;
  watching: string | null;
  onWatch: (userID: string | null) => void;
  onSelectGuild: (g: Guild) => void;
  onSelectChannel: (c: Channel) => void;
  onGuildsChanged: () => void;
  unread: Record<string, boolean>;
}

type Group = { category: Channel | null; channels: Channel[] };

const emptyScreens: ScreenState = {
  sharing: false,
  sound: false,
  local: null,
  remote: [],
  audible: [],
  dropped: [],
  sizes: {},
  quality: "smooth",
};

function inOrder(channels: Channel[]): Channel[] {
  return [...channels].sort((a, b) =>
    a.position === b.position ? a.id.localeCompare(b.id) : a.position - b.position,
  );
}

function groupByCategory(channels: Channel[]): Group[] {
  const categories = inOrder(channels.filter((c) => c.kind === "category"));
  const rest = channels.filter((c) => c.kind !== "category");

  const loose = inOrder(
    rest.filter((c) => !c.parent_id || !categories.some((k) => k.id === c.parent_id)),
  );
  const groups: Group[] = loose.length > 0 ? [{ category: null, channels: loose }] : [];

  for (const category of categories) {
    groups.push({ category, channels: inOrder(rest.filter((c) => c.parent_id === category.id)) });
  }
  return groups;
}

export function Sidebar({
  guilds,
  channels,
  activeGuild,
  activeChannel,
  selfID,
  watching,
  onWatch,
  onSelectGuild,
  onSelectChannel,
  onGuildsChanged,
  unread,
}: Props) {
  const [people, setPeople] = useState<SessionState>(emptySession);
  const [screens, setScreens] = useState<ScreenState>(emptyScreens);
  const [speaking, setSpeaking] = useState<Record<string, boolean>>({});
  const [folded, setFolded] = useState<Record<string, boolean>>({});
  const [guildMenu, setGuildMenu] = useState(false);
  const [makingServer, setMakingServer] = useState(false);
  const [makingChannel, setMakingChannel] = useState<Channel | null | false>(false);
  const [invitesOpen, setInvitesOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [person, setPerson] = useState<string | null>(null);

  useEffect(() => session.onChange(setPeople), []);
  useEffect(() => voice.onScreenChange(setScreens), []);
  useEffect(() => voice.onSpeakingChange(setSpeaking), []);

  const closeGuildMenu = useCallback(() => setGuildMenu(false), []);
  const guildAnchor = useDismiss<HTMLDivElement>(guildMenu, closeGuildMenu);

  const held = activeGuild ? people.guildAllows[activeGuild.id] : undefined;
  const mayManageChannels = allows(held, MANAGE_CHANNELS);
  const mayInvite = allows(held, CREATE_INVITE);
  const maySeeSettings =
    allows(held, MANAGE_ROLES) || allows(held, BAN_MEMBERS) || allows(held, MANAGE_GUILD);

  const groups = groupByCategory(channels);
  const roster = activeGuild ? (people.membersByGuild[activeGuild.id] ?? []) : [];
  const online = roster.filter((id) => people.online[id]).length;
  const live = new Set(
    screens.remote.map((s) => s.userID).filter((id): id is string => Boolean(id)),
  );

  const move = async (channelID: string, to: number) => {
    try {
      await api.moveChannel(channelID, to);
    } catch {}
  };

  function voiceRow(channel: Channel, id: string) {
    const classes = ["voice-person"];
    if (watching === id) classes.push("watching");
    if (!people.online[id] && id !== selfID) classes.push("quiet");

    return (
      <button
        key={id}
        className={classes.join(" ")}
        onContextMenu={(e) => {
          e.preventDefault();
          if (id !== selfID) setPerson(id);
        }}
        onClick={() => {
          if (!live.has(id)) return;
          voice.watchScreen(id, true);
          onSelectChannel(channel);
          onWatch(id);
        }}
      >
        <Avatar
          name={session.nameOf(id)}
          imageKey={people.avatars[id]}
          mine={id === selfID}
          className={speaking[id] ? "speaking" : undefined}
        />
        <span className="grow clip">{session.nameOf(id)}</span>
        {speaking[id] && (
          <span className="bars">
            <span />
            <span />
          </span>
        )}
        {people.mutedInVoice[id] && <Icon name="microphone-slash" size={13} />}
        {people.deafenedInVoice[id] && <Icon name="speaker-simple-slash" size={13} />}
        {live.has(id) && <span className="live">LIVE</span>}
        {id === selfID && <span className="you-tag">you</span>}
      </button>
    );
  }

  return (
    <>
      {makingServer && (
        <NewServer
          onClose={() => setMakingServer(false)}
          onMade={() => {
            setMakingServer(false);
            onGuildsChanged();
          }}
        />
      )}

      {makingChannel !== false && activeGuild && (
        <NewChannel
          guild={activeGuild}
          parent={makingChannel}
          taken={channels.length}
          onClose={() => setMakingChannel(false)}
        />
      )}

      {invitesOpen && activeGuild && (
        <Invites guild={activeGuild} onClose={() => setInvitesOpen(false)} />
      )}

      {settingsOpen && activeGuild && (
        <ServerSettings
          guild={activeGuild}
          channels={channels}
          onClose={() => setSettingsOpen(false)}
        />
      )}

      {person && (
        <PersonMenu
          userID={person}
          live={live.has(person)}
          onClose={() => setPerson(null)}
        />
      )}

      <nav className="rail" aria-label="Servers">
        {guilds.map((g) => (
          <button
            key={g.id}
            className={g.id === activeGuild?.id ? "rail-server active" : "rail-server"}
            title={g.name}
            onClick={() => onSelectGuild(g)}
          >
            <Avatar name={g.name} imageKey={g.icon_key} />
          </button>
        ))}
        <button
          className="rail-server rail-add"
          title="A place for the group"
          onClick={() => setMakingServer(true)}
        >
          <Icon name="plus" size={17} />
        </button>
        <span className="rail-fade" />
      </nav>

      <aside className="card channels">
        <div className="anchor" ref={guildAnchor}>
          <button
            className="guild-head"
            disabled={!activeGuild}
            aria-expanded={guildMenu}
            onClick={() => setGuildMenu(!guildMenu)}
          >
            <span className="grow">
              <span className="guild-head-name clip">{activeGuild?.name ?? "No server"}</span>
              <span className="guild-head-meta">
                {activeGuild
                  ? `${roster.length} member${roster.length === 1 ? "" : "s"} · ${online} online`
                  : "Make one, or step into one you were given a code for"}
              </span>
            </span>
            {activeGuild && <Icon name="caret-down" size={15} />}
          </button>

          {guildMenu && activeGuild && (
            <div className="menu below-left">
              <div className="menu-title">The server</div>
              <button
                className="menu-item"
                disabled={!mayInvite}
                onClick={() => {
                  closeGuildMenu();
                  setInvitesOpen(true);
                }}
              >
                <Icon name="user-plus" size={16} />
                Invite people
                {!mayInvite && <span className="menu-item-hint">not yours to give</span>}
              </button>
              <button
                className="menu-item"
                disabled={!maySeeSettings}
                onClick={() => {
                  closeGuildMenu();
                  setSettingsOpen(true);
                }}
              >
                <Icon name="gear-six" size={16} />
                Server settings
              </button>
              <div className="menu-separator" />
              <button
                className="menu-item"
                disabled={!mayManageChannels}
                onClick={() => {
                  closeGuildMenu();
                  setMakingChannel(null);
                }}
              >
                <Icon name="plus-circle" size={16} />
                Add a channel
              </button>
            </div>
          )}
        </div>

        <div className="channel-list">
          {groups.map((group) => {
            const key = group.category?.id ?? "loose";
            const shut = Boolean(group.category && folded[group.category.id]);

            return (
              <div key={key} className="channel-group">
                {group.category && (
                  <button
                    className={shut ? "category folded" : "category"}
                    onClick={() =>
                      setFolded((prev) => ({
                        ...prev,
                        [group.category!.id]: !prev[group.category!.id],
                      }))
                    }
                  >
                    <Icon name="caret-down" size={11} />
                    {group.category.name}
                  </button>
                )}

                {!shut &&
                  group.channels.map((c, at) => {
                    const here = people.inVoice[c.id] ?? [];
                    const classes = ["channel"];
                    if (c.id === activeChannel?.id) classes.push("active");
                    else if (c.kind === "voice" && here.length > 0) classes.push("busy");
                    else if (unread[c.id]) classes.push("unread");

                    return (
                      <div key={c.id} className="channel-slot">
                        <button className={classes.join(" ")} onClick={() => onSelectChannel(c)}>
                          <Icon name={c.kind === "voice" ? "speaker-high" : "hash"} size={15} />
                          <span className="clip">{c.name}</span>

                          {c.kind === "voice" && here.length > 0 && (
                            <span className="channel-count">{here.length}</span>
                          )}
                          {c.kind !== "voice" && unread[c.id] && c.id !== activeChannel?.id && (
                            <span className="unread-dot" aria-label="unread messages" />
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

                        {c.kind === "voice" && here.length > 0 && (
                          <div className="voice-roster">
                            {here.map((id) => voiceRow(c, id))}
                          </div>
                        )}
                      </div>
                    );
                  })}
              </div>
            );
          })}

          {activeGuild && channels.length === 0 && (
            <div className="note">Nothing here yet. Add a channel from the server name above.</div>
          )}
        </div>
      </aside>
    </>
  );
}
