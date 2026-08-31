import { useCallback, useEffect, useMemo, useState } from "react";
import { api, ApiError } from "./api";
import { gateway } from "./gateway";
import { joinsMuted, setJoinsMuted } from "./audioPrefs";
import {
  ADD_REACTIONS,
  allows,
  BAN_MEMBERS,
  CREATE_INVITE,
  KICK_MEMBERS,
  MANAGE_CHANNELS,
  MANAGE_GUILD,
  MANAGE_MESSAGES,
  SEND_MESSAGES,
  VIEW_CHANNEL,
} from "./permissions";
import { emptySession, nameOf, session, type SessionState } from "./session";
import { voice, type ScreenQualityID, type ScreenState, type Speaking } from "./voice";
import type {
  Attachment,
  Channel,
  Guild,
  GuildRemoval,
  Message,
  MessageReaction,
  User,
} from "./types/events.gen";

import { Auth } from "./screens/Auth";
import { EmojiPalette } from "./screens/EmojiPalette";
import { ReactionPalette } from "./screens/ReactionPalette";
import { GoLive } from "./screens/GoLive";
import { Lightbox } from "./screens/Lightbox";
import { NewServer } from "./screens/NewServer";
import { ProfileSettings } from "./screens/ProfileSettings";
import { ServerSettings } from "./screens/ServerSettings";
import {
  CategoryMenu,
  ChannelListMenu,
  ChannelMenu,
  MessageMenu,
  PersonMenu,
  VoiceMemberMenu,
} from "./screens/menus";
import { Channels, type ChannelMenuTarget } from "./shell/Channels";
import { Members } from "./shell/Members";
import { Room } from "./shell/Room";
import { ServerRail } from "./shell/ServerRail";
import { StreamStage } from "./shell/StreamStage";
import { VoiceRoom } from "./shell/VoiceRoom";
import { YourBar } from "./shell/YourBar";
import { fileURL } from "./ui/Avatar";
import { Button } from "./ui/Button";
import { Icon } from "./ui/Icon";
import { Sheet } from "./ui/Sheet";
import { Toggle } from "./ui/Toggle";
import { ContextMenu, type Anchor } from "./ui/ContextMenu";
import { MenuItem, MenuSeparator } from "./ui/Menu";

type Menu =
  | { kind: "channel"; at: Anchor; channel: Channel }
  | { kind: "category"; at: Anchor }
  | { kind: "list"; at: Anchor }
  | { kind: "person"; at: Anchor; userID: string }
  | { kind: "voice"; at: Anchor; userID: string }
  | { kind: "message"; at: Anchor; message: Message };

type Confirm = { action: "kick" | "ban"; userID: string };

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [booting, setBooting] = useState(true);

  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [activeGuild, setActiveGuild] = useState<Guild | null>(null);
  const [activeChannel, setActiveChannel] = useState<Channel | null>(null);
  const [state, setState] = useState<SessionState>(emptySession);

  const [messages, setMessages] = useState<Message[]>([]);
  const [typing, setTyping] = useState<string[]>([]);

  const [screens, setScreens] = useState<ScreenState>({
    sharing: false,
    sound: false,
    remote: [],
    audible: [],
    dropped: [],
    sizes: {},
    quality: "smooth",
  });
  const [speaking, setSpeaking] = useState<Speaking>({});
  const [muted, setMuted] = useState(joinsMuted);
  const [deafened, setDeafened] = useState(false);
  const [suppressing, setSuppressing] = useState(() => voice.suppressing);
  const [callChannel, setCallChannel] = useState<Channel | null>(null);
  const [watching, setWatching] = useState<string | null>(null);

  const syncVoiceFlags = useCallback(() => {
    setMuted(voice.inCall ? voice.muted : joinsMuted());
    setDeafened(voice.deafened);
  }, []);

  const [newServer, setNewServer] = useState(false);
  const [serverSettings, setServerSettings] = useState(false);
  const [profileSettings, setProfileSettings] = useState(false);
  const [goLive, setGoLive] = useState(false);
  const [lightbox, setLightbox] = useState<{ file: Attachment; message: Message } | null>(null);
  const [menu, setMenu] = useState<Menu | null>(null);
  const [confirm, setConfirm] = useState<Confirm | null>(null);
  const [emoji, setEmoji] = useState<{
    messageID: string | null;
    right: number;
    bottom: number;
    full: boolean;
  } | null>(null);
  const [newChannel, setNewChannel] = useState<"text" | "voice" | "category" | null>(null);
  const [channelName, setChannelName] = useState("");
  const [privateChannel, setPrivateChannel] = useState(false);
  const [editingChannel, setEditingChannel] = useState<{ channel: Channel; name: string } | null>(null);
  const [droppingChannel, setDroppingChannel] = useState<Channel | null>(null);
  const [renaming, setRenaming] = useState<{ userID: string; name: string } | null>(null);
  const [serverMenu, setServerMenu] = useState<Anchor | null>(null);
  const [editing, setEditing] = useState<Message | null>(null);
  const [editDraft, setEditDraft] = useState("");
  const [leaving, setLeaving] = useState(false);
  const [invite, setInvite] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [draftEmoji, setDraftEmoji] = useState<{ char: string; n: number } | null>(null);

  useEffect(() => {
    if (!emoji) return;
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setEmoji(null);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [emoji]);

  useEffect(
    () =>
      api.onSessionEnded(() => {
        gateway.close();
        session.forget();
        setUser(null);
        setNotice(null);
      }),
    [],
  );

  const applyReaction = useCallback(
    (messageID: string, emoji: string, add: boolean, mine: boolean) => {
      setMessages((prev) =>
        prev.map((m) => {
          if (m.id !== messageID) return m;
          const held = m.reactions.find((r) => r.emoji === emoji);
          if (add) {
            if (!held) {
              return { ...m, reactions: [...m.reactions, { emoji, count: 1, mine }] };
            }
            if (mine && held.mine) return m;
            return {
              ...m,
              reactions: m.reactions.map((r) =>
                r.emoji === emoji
                  ? { ...r, count: r.count + 1, mine: r.mine || mine }
                  : r,
              ),
            };
          }
          if (!held) return m;
          return {
            ...m,
            reactions: m.reactions
              .map((r) =>
                r.emoji === emoji
                  ? { ...r, count: r.count - 1, mine: mine ? false : r.mine }
                  : r,
              )
              .filter((r) => r.count > 0),
          };
        }),
      );
    },
    [],
  );

  const react = useCallback(
    (messageID: string, emoji: string) => {
      applyReaction(messageID, emoji, true, true);
      void api.react(messageID, emoji).catch(() => {
        applyReaction(messageID, emoji, false, true);
        setNotice("That reaction was not saved.");
      });
    },
    [applyReaction],
  );

  const unreact = useCallback(
    (messageID: string, emoji: string) => {
      applyReaction(messageID, emoji, false, true);
      void api.unreact(messageID, emoji).catch(() => {
        applyReaction(messageID, emoji, true, true);
        setNotice("That reaction was not removed.");
      });
    },
    [applyReaction],
  );

  useEffect(() => session.onChange(setState), []);
  useEffect(() => voice.onScreenChange(setScreens), []);
  useEffect(() => voice.onSpeakingChange(setSpeaking), []);

  useEffect(() => {
    session.reading(activeChannel?.kind === "text" ? activeChannel.id : null);
  }, [activeChannel]);

  useEffect(() => {
    if (!api.authenticated) {
      setBooting(false);
      return;
    }
    api
      .me()
      .then(setUser)
      .catch(() => api.clear())
      .finally(() => setBooting(false));
  }, []);

  useEffect(() => {
    if (!user) return;
    gateway.connect(api.token!);
    return () => gateway.close();
  }, [user]);

  const loadGuilds = useCallback(async () => {
    const list = await api.guilds();
    setGuilds(list);
    setActiveGuild((current) => {
      if (current && list.some((g) => g.id === current.id)) return current;
      return list[0] ?? null;
    });
  }, []);

  useEffect(() => {
    if (!user) return;
    void loadGuilds();
    return gateway.on("GUILD_CREATE", () => void loadGuilds());
  }, [user, loadGuilds]);

  useEffect(() => {
    if (!user) return;
    const forgetUser = gateway.on("USER_UPDATE", (payload) => {
      const who = payload as User;
      setUser((held) => (held && held.id === who.id ? { ...held, ...who } : held));
    });
    const forgetGuild = gateway.on("GUILD_UPDATE", (payload) => {
      const changed = payload as Guild;
      setGuilds((prev) => prev.map((g) => (g.id === changed.id ? changed : g)));
      setActiveGuild((held) => (held?.id === changed.id ? changed : held));
    });
    return () => {
      forgetUser();
      forgetGuild();
    };
  }, [user]);

  useEffect(() => {
    if (!user) return;
    return gateway.on("GUILD_REMOVE", (payload) => {
      const gone = payload as GuildRemoval;
      setGuilds((prev) => {
        const left = prev.filter((g) => g.id !== gone.guild_id);
        setActiveGuild((current) =>
          current?.id === gone.guild_id ? (left[0] ?? null) : current,
        );
        return left;
      });
    });
  }, [user]);

  useEffect(() => {
    if (!activeGuild) {
      setChannels([]);
      setActiveChannel(null);
      return;
    }
    let dropped = false;
    void api.channels(activeGuild.id).then((list) => {
      if (dropped) return;
      setChannels(list);
      setActiveChannel(list.find((c) => c.kind === "text") ?? list[0] ?? null);
    });
    return () => {
      dropped = true;
    };
  }, [activeGuild]);

  useEffect(() => {
    if (!activeGuild) return;
    const forgetCreate = gateway.on("CHANNEL_CREATE", (payload) => {
      const channel = payload as Channel;
      if (channel.guild_id !== activeGuild.id) return;
      setChannels((prev) => (prev.some((c) => c.id === channel.id) ? prev : [...prev, channel]));
    });
    const forgetUpdate = gateway.on("CHANNEL_UPDATE", (payload) => {
      const channel = payload as Channel;
      if (channel.guild_id !== activeGuild.id) return;
      setChannels((prev) => prev.map((c) => (c.id === channel.id ? channel : c)));
    });
    const forgetDelete = gateway.on("CHANNEL_DELETE", (payload) => {
      const channel = payload as Channel;
      if (channel.guild_id !== activeGuild.id) return;
      setChannels((prev) =>
        prev
          .filter((c) => c.id !== channel.id)
          .map((c) => (c.parent_id === channel.id ? { ...c, parent_id: undefined } : c)),
      );
      setActiveChannel((held) => (held?.id === channel.id ? null : held));
    });
    return () => {
      forgetCreate();
      forgetUpdate();
      forgetDelete();
    };
  }, [activeGuild]);

  const reload = useCallback(async () => {
    if (!activeChannel || activeChannel.kind !== "text") {
      setMessages([]);
      return;
    }
    const history = await api.messages(activeChannel.id);
    setMessages(history.slice().reverse());
  }, [activeChannel]);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    if (!activeChannel) return;
    const created = gateway.on("MESSAGE_CREATE", (payload) => {
      const message = payload as Message;
      if (message.channel_id !== activeChannel.id) return;
      setMessages((prev) => (prev.some((m) => m.id === message.id) ? prev : [...prev, message]));
    });
    const updated = gateway.on("MESSAGE_UPDATE", (payload) => {
      const message = payload as Message;
      setMessages((prev) => prev.map((m) => (m.id === message.id ? message : m)));
    });
    const removed = gateway.on("MESSAGE_DELETE", (payload) => {
      const gone = payload as { message_id: string };
      setMessages((prev) => prev.filter((m) => m.id !== gone.message_id));
    });
    const reacted = gateway.on("MESSAGE_REACTION_ADD", (payload) => {
      const hit = payload as MessageReaction;
      if (hit.channel_id !== activeChannel.id) return;
      if (hit.user_id === user?.id) return;
      applyReaction(hit.message_id, hit.emoji, true, false);
    });

    const unreacted = gateway.on("MESSAGE_REACTION_REMOVE", (payload) => {
      const hit = payload as MessageReaction;
      if (hit.channel_id !== activeChannel.id) return;
      if (hit.user_id === user?.id) return;
      applyReaction(hit.message_id, hit.emoji, false, false);
    });

    const typed = gateway.on("TYPING_START", (payload) => {
      const who = payload as { channel_id: string; user_id: string };
      if (who.channel_id !== activeChannel.id || who.user_id === user?.id) return;
      setTyping((prev) => (prev.includes(who.user_id) ? prev : [...prev, who.user_id]));
      setTimeout(() => setTyping((prev) => prev.filter((id) => id !== who.user_id)), 6000);
    });
    return () => {
      created();
      updated();
      removed();
      reacted();
      unreacted();
      typed();
    };
  }, [activeChannel, user, applyReaction]);

  const avatarURL = useCallback(
    (id: string) => fileURL(state.avatars[id] ?? null),
    [state.avatars],
  );

  const nameFor = useCallback(
    (id: string) => nameOf(state, activeGuild?.id ?? null, id),
    [state, activeGuild],
  );

  const permissions = activeGuild ? (state.guildAllows[activeGuild.id] ?? 0) : 0;
  const channelAllows = activeChannel ? state.channelAllows[activeChannel.id] : undefined;
  const effective = channelAllows ?? permissions;

  const live = useMemo(() => {
    const set = new Set(
      screens.remote.map((screen) => screen.userID).filter(Boolean) as string[],
    );
    if (screens.sharing && user) set.add(user.id);
    return set;
  }, [screens, user]);

  const watchedStream = watching
    ? (screens.remote.find((screen) => screen.userID === watching)?.stream ?? null)
    : null;

  function closeCreate() {
    setNewChannel(null);
    setChannelName("");
    setPrivateChannel(false);
  }

  async function makeChannel(guildID: string) {
    const kind = newChannel;
    const wanted = channelName.trim();
    if (!kind || !wanted) return;
    const shut = privateChannel && kind !== "category";
    closeCreate();
    const made = await api.createChannel(guildID, wanted, kind, channels.length);
    if (shut) {
      const everyone = (await api.roles(guildID)).find((role) => role.is_default);
      if (everyone) {
        await api.setOverwrite(made.id, everyone.id, 0, VIEW_CHANNEL).catch(() => undefined);
      }
    }
    setChannels(await api.channels(guildID));
  }

  function startShare() {
    setGoLive(true);
  }

  async function joinVoice(channel: Channel) {
    if (!user) return;
    setCallChannel(channel);
    await voice.join(channel.id, user.id);
    syncVoiceFlags();
  }

  async function hangUp() {
    await voice.leave();
    setCallChannel(null);
    setWatching(null);
    syncVoiceFlags();
  }

  if (booting) return <div className="auth" />;
  if (!user) return <Auth onSignedIn={setUser} />;

  const showingVoice = activeChannel?.kind === "voice";

  return (
    <div className="app">
      {notice && (
        <div className="notice" role="status">
          <span>{notice}</span>
          <button type="button" className="notice-dismiss" onClick={() => setNotice(null)}>
            Dismiss
          </button>
        </div>
      )}
      <div className="left-column">
        <div className="rail-and-channels">
          <ServerRail
            guilds={guilds}
            activeID={activeGuild?.id ?? null}
            iconURL={(guild) => fileURL(guild.icon_key)}
            onPick={setActiveGuild}
            onAdd={() => setNewServer(true)}
            onReorder={async (ids) => {
              const before = guilds;
              const byID = new Map(before.map((g) => [g.id, g]));
              setGuilds(ids.map((id) => byID.get(id)!).filter(Boolean));
              try {
                setGuilds(await api.reorderGuilds(ids));
              } catch (cause) {
                setGuilds(before);
                setNotice(
                  cause instanceof ApiError
                    ? `The new order was not saved: ${cause.message}`
                    : "The new order was not saved.",
                );
              }
            }}
          />
          {activeGuild && (
            <Channels
              guild={activeGuild}
              channels={channels}
              activeChannelID={activeChannel?.id ?? null}
              state={state}
              nameFor={nameFor}
              meID={user.id}
              live={live}
              speaking={speaking}
              avatarURL={avatarURL}
              onPick={(channel) => {
                setActiveChannel(channel);
                if (channel.kind === "voice" && callChannel?.id !== channel.id) {
                  void joinVoice(channel);
                }
              }}
              onOpenServerMenu={(event) =>
                setServerMenu({ x: event.clientX, y: event.clientY + 6 })
              }
              onOpenMenu={(target: ChannelMenuTarget, event) => {
                event.preventDefault();
                const at = { x: event.clientX, y: event.clientY };
                if (target.kind === "channel") {
                  setMenu({ kind: "channel", at, channel: target.channel });
                } else if (target.kind === "category") {
                  setMenu({ kind: "category", at });
                } else if (allows(permissions, MANAGE_CHANNELS)) {
                  setMenu({ kind: "list", at });
                }
              }}
              onOpenVoiceMember={(userID, _channelID, event) => {
                event.preventDefault();
                setMenu({ kind: "voice", at: { x: event.clientX, y: event.clientY }, userID });
              }}
              canManage={allows(permissions, MANAGE_CHANNELS)}
              onMoveChannel={async (move) => {
                const before = channels;
                const here = before.find((c) => c.id === move.channelID)?.parent_id ?? null;
                const reparent = here !== move.parentID;
                setChannels((prev) =>
                  prev.map((c) =>
                    c.id === move.channelID
                      ? { ...c, parent_id: move.parentID ?? undefined, position: move.position }
                      : c,
                  ),
                );
                try {
                  await api.moveChannel(
                    move.channelID,
                    move.position,
                    move.parentID,
                    reparent,
                  );
                  setChannels(await api.channels(activeGuild.id));
                } catch (cause) {
                  setChannels(before);
                  setNotice(
                    cause instanceof ApiError
                      ? `That channel was not moved: ${cause.message}`
                      : "That channel was not moved.",
                  );
                }
              }}
            />
          )}
        </div>

        <YourBar
          me={user.username}
          meAvatarURL={fileURL(user.avatar_key)}
          presence={callChannel ? "In voice" : "Online"}
          call={
            callChannel
              ? {
                  channelName: callChannel.name,
                  status: "Voice connected",
                  quality: "good",
                  sharing: screens.sharing,
                }
              : null
          }
          watching={
            watching
              ? {
                  userID: watching,
                  name: nameFor(watching),
                  avatarURL: avatarURL(watching),
                }
              : null
          }
          muted={muted}
          deafened={deafened}
          onToggleMute={() => {
            if (voice.inCall) {
              voice.toggleMute();
            } else {
              const next = !joinsMuted();
              setJoinsMuted(next);
              if (!next && voice.deafened) voice.toggleDeafen();
            }
            syncVoiceFlags();
          }}
          onToggleDeafen={() => {
            voice.toggleDeafen();
            if (!voice.inCall) setJoinsMuted(voice.deafened);
            syncVoiceFlags();
          }}
          onOpenSettings={() => setProfileSettings(true)}
          onToggleShare={() => (screens.sharing ? void voice.stopScreenShare() : startShare())}
          onHangUp={() => void hangUp()}
          suppressing={suppressing}
          onSuppression={(on) => {
            setSuppressing(on);
            void voice.setSuppression(on).then(setSuppressing);
          }}
          onStopWatching={() => {
            if (watching) voice.watchScreen(watching, false);
            setWatching(null);
          }}
        />
      </div>

      {watching && watchedStream ? (
        <StreamStage
          name={nameFor(watching)}
          avatarURL={avatarURL(watching)}
          stream={watchedStream}
          userID={watching}
          screens={screens}
          muted={muted}
          deafened={deafened}
          onToggleMute={() => {
            voice.toggleMute();
            syncVoiceFlags();
          }}
          onToggleDeafen={() => {
            voice.toggleDeafen();
            syncVoiceFlags();
          }}
          onHangUp={() => void hangUp()}
          onGoLive={startShare}
          onStopSharing={() => void voice.stopScreenShare()}
          onStopWatching={() => {
            voice.watchScreen(watching, false);
            setWatching(null);
          }}
        />
      ) : showingVoice && activeChannel ? (
        <VoiceRoom
          channel={activeChannel}
          inCall={state.inVoice[activeChannel.id] ?? []}
          state={state}
          nameFor={nameFor}
          meID={user.id}
          speaking={speaking}
          screens={screens}
          muted={muted}
          deafened={deafened}
          avatarURL={avatarURL}
          onToggleMute={() => {
            voice.toggleMute();
            syncVoiceFlags();
          }}
          onToggleDeafen={() => {
            voice.toggleDeafen();
            syncVoiceFlags();
          }}
          onHangUp={() => void hangUp()}
          onGoLive={startShare}
          onStopSharing={() => void voice.stopScreenShare()}
          onWatch={(userID) => {
            voice.watchScreen(userID, true);
            setWatching(userID);
          }}
          onOpenMember={(userID, event) => {
            event.preventDefault();
            setMenu({ kind: "voice", at: { x: event.clientX, y: event.clientY }, userID });
          }}
        />
      ) : activeChannel ? (
        <Room
          key={activeChannel.id}
          channel={activeChannel}
          messages={messages}
          nameFor={nameFor}
          meID={user.id}
          typing={typing.map(nameFor)}
          canSend={allows(effective, SEND_MESSAGES)}
          canReact={allows(effective, ADD_REACTIONS)}
          avatarURL={avatarURL}
          onReact={react}
          onUnreact={unreact}
          onOpenEmoji={(messageID, anchor) =>
            setEmoji({
              messageID,
              right: Math.max(12, window.innerWidth - anchor.right),
              bottom: window.innerHeight - anchor.top + 8,
              full: messageID === null,
            })
          }
          onOpenMessageMenu={(message, event) => {
            event.preventDefault();
            setMenu({
              kind: "message",
              at: { x: event.clientX, y: event.clientY },
              message,
            });
          }}
          onOpenImage={(file, message) => setLightbox({ file, message })}
          insert={draftEmoji}
          onSent={() => void reload()}
        />
      ) : (
        <section className="room panel" />
      )}

      {!showingVoice && !watching && activeGuild && (
        <Members
          ids={state.membersByGuild[activeGuild.id] ?? []}
          state={state}
          nameFor={nameFor}
          meID={user.id}
          live={live}
          avatarURL={avatarURL}
          onOpenMenu={(userID, event) => {
            event.preventDefault();
            if (!allows(permissions, KICK_MEMBERS) && !allows(permissions, BAN_MEMBERS)) {
              return;
            }
            setMenu({ kind: "person", at: { x: event.clientX, y: event.clientY }, userID });
          }}
        />
      )}

      {serverMenu && activeGuild && (
        <ContextMenu at={serverMenu} onClose={() => setServerMenu(null)}>
          <MenuItem
            icon="gear-six"
            label="Server settings"
            onClick={() => {
              setServerMenu(null);
              setServerSettings(true);
            }}
          />
          <MenuItem
            icon="plus"
            label="Invite people"
            disabled={!allows(permissions, CREATE_INVITE)}
            hint={allows(permissions, CREATE_INVITE) ? undefined : "Not allowed"}
            onClick={async () => {
              setServerMenu(null);
              const made = await api.createInvite(activeGuild.id);
              setInvite(made.code);
            }}
          />
          <MenuSeparator />
          <MenuItem
            icon="arrow-left"
            kind="danger"
            label="Leave server"
            disabled={activeGuild.owner_id === user.id}
            hint={
              activeGuild.owner_id === user.id
                ? "You own it"
                : undefined
            }
            onClick={() => {
              setServerMenu(null);
              setLeaving(true);
            }}
          />
        </ContextMenu>
      )}

      {invite && (
        <Sheet
          title="Invite people"
          subtitle="Anyone with this code can step in. Revoke it from Settings → Links."
          onClose={() => setInvite(null)}
        >
          <label className="field">
            <span className="field-label">Invite code</span>
            <input className="input" value={invite} readOnly />
          </label>
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setInvite(null)}>
              Done
            </Button>
            <Button onClick={() => void navigator.clipboard.writeText(invite)}>Copy</Button>
          </div>
        </Sheet>
      )}

      {leaving && activeGuild && (
        <Sheet
          title={`Leave ${activeGuild.name}?`}
          subtitle="What you wrote stays where it is. You will need a new invite to come back."
          onClose={() => setLeaving(false)}
        >
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setLeaving(false)}>
              Never mind
            </Button>
            <Button
              kind="danger"
              onClick={async () => {
                await api.leaveGuild(activeGuild.id);
                setLeaving(false);
                await loadGuilds();
              }}
            >
              Leave it
            </Button>
          </div>
        </Sheet>
      )}

      {editing && (
        <Sheet
          title="Edit message"
          subtitle="Everyone sees the change, and it is marked as edited."
          onClose={() => setEditing(null)}
        >
          <label className="field">
            <span className="field-label">Message</span>
            <textarea
              className="textarea"
              defaultValue={editing.content}
              autoFocus
              onChange={(event) => setEditDraft(event.target.value)}
            />
          </label>
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setEditing(null)}>
              Cancel
            </Button>
            <Button
              onClick={async () => {
                const next = (editDraft || editing.content).trim();
                if (next) await api.editMessage(editing.id, next);
                setEditing(null);
                setEditDraft("");
              }}
            >
              Save
            </Button>
          </div>
        </Sheet>
      )}

      {newServer && (
        <NewServer
          onClose={() => setNewServer(false)}
          onArrived={(guild) => {
            setNewServer(false);
            setActiveGuild(guild);
            void loadGuilds();
          }}
        />
      )}

      {serverSettings && activeGuild && (
        <ServerSettings
          guild={activeGuild}
          channels={channels}
          permissions={permissions}
          iconURL={fileURL(activeGuild.icon_key)}
          online={state.online}
          onClose={() => setServerSettings(false)}
          onChanged={() => void loadGuilds()}
        />
      )}

      {profileSettings && (
        <ProfileSettings
          user={user}
          avatarURL={fileURL(user.avatar_key)}
          onClose={() => {
            setProfileSettings(false);
            syncVoiceFlags();
          }}
          onChanged={() => void api.me().then(setUser)}
          onSignOut={() => {
            void api.logout();
            setUser(null);
          }}
          onDeleteAccount={() => setProfileSettings(false)}
        />
      )}

      {goLive && (
        <GoLive
          quality={screens.quality}
          onQuality={(id: ScreenQualityID) => voice.setScreenQuality(id)}
          onStart={(sourceID: string, audio: boolean) => {
            setGoLive(false);
            void voice.startScreenShare(sourceID, audio).then((started) => {
              if (!started) setNotice("That could not be shared.");
            });
          }}
          onClose={() => setGoLive(false)}
        />
      )}

      {lightbox && (
        <Lightbox
          attachment={lightbox.file}
          uploader={nameFor(lightbox.message.author.id)}
          uploaderAvatarURL={avatarURL(lightbox.message.author.id)}
          postedAt={new Date(lightbox.message.created_at).toLocaleString()}
          onClose={() => setLightbox(null)}
        />
      )}

      {emoji && (
        <div className="emoji-layer" onPointerDown={() => setEmoji(null)}>
          <div
            className="emoji-anchor"
            style={{ right: emoji.right, bottom: emoji.bottom }}
            onPointerDown={(event) => event.stopPropagation()}
          >
            {emoji.full ? (
              <EmojiPalette
                onPick={(char) => {
                  if (emoji.messageID) react(emoji.messageID, char);
                  else setDraftEmoji((was) => ({ char, n: (was?.n ?? 0) + 1 }));
                  setEmoji(null);
                }}
              />
            ) : (
              <ReactionPalette
                onPick={(char) => {
                  if (emoji.messageID) react(emoji.messageID, char);
                  setEmoji(null);
                }}
                onMore={() => setEmoji({ ...emoji, full: true })}
              />
            )}
          </div>
        </div>
      )}

      {menu?.kind === "channel" && (
        <ChannelMenu
          at={menu.at}
          channel={menu.channel}
          canManage={allows(effective, MANAGE_CHANNELS)}
          onEdit={() => setEditingChannel({ channel: menu.channel, name: menu.channel.name })}
          onDelete={() => setDroppingChannel(menu.channel)}
          onClose={() => setMenu(null)}
        />
      )}

      {menu?.kind === "message" && (
        <MessageMenu
          at={menu.at}
          mine={menu.message.author.id === user.id}
          canDelete={menu.message.author.id === user.id || allows(permissions, MANAGE_MESSAGES)}
          onEdit={() => setEditing(menu.message)}
          onDelete={() => void api.deleteMessage(menu.message.id)}
          onCopy={() => void navigator.clipboard.writeText(menu.message.content)}
          onClose={() => setMenu(null)}
        />
      )}

      {menu?.kind === "category" && (
        <CategoryMenu
          at={menu.at}
          canManage={allows(permissions, MANAGE_CHANNELS)}
          onNewChannel={() => setNewChannel("text")}
          onNewVoiceChannel={() => setNewChannel("voice")}
          onClose={() => setMenu(null)}
        />
      )}

      {menu?.kind === "list" && (
        <ChannelListMenu
          at={menu.at}
          canManage={allows(permissions, MANAGE_CHANNELS)}
          onNewChannel={() => setNewChannel("text")}
          onNewCategory={() => setNewChannel("category")}
          onClose={() => setMenu(null)}
        />
      )}

      {menu?.kind === "person" && (
        <PersonMenu
          at={menu.at}
          canKick={allows(permissions, KICK_MEMBERS)}
          canBan={allows(permissions, BAN_MEMBERS)}
          onKick={() => setConfirm({ action: "kick", userID: menu.userID })}
          onBan={() => setConfirm({ action: "ban", userID: menu.userID })}
          onClose={() => setMenu(null)}
        />
      )}

      {menu?.kind === "voice" && (
        <VoiceMemberMenu
          at={menu.at}
          userID={menu.userID}
          name={nameFor(menu.userID)}
          avatarURL={avatarURL(menu.userID)}
          live={live.has(menu.userID)}
          canRename={menu.userID === user.id || allows(permissions, MANAGE_GUILD)}
          onRename={() =>
            setRenaming({ userID: menu.userID, name: nameFor(menu.userID) })
          }
          canKick={allows(permissions, KICK_MEMBERS)}
          canBan={allows(permissions, BAN_MEMBERS)}
          onKick={() => setConfirm({ action: "kick", userID: menu.userID })}
          onBan={() => setConfirm({ action: "ban", userID: menu.userID })}
          onClose={() => setMenu(null)}
        />
      )}

      {confirm && activeGuild && (
        <Sheet
          title={confirm.action === "kick" ? "Kick member" : "Ban member"}
          subtitle={
            confirm.action === "kick"
              ? `${nameFor(confirm.userID)} can come back with a new invite. What they wrote stays.`
              : `${nameFor(confirm.userID)} is kept out by account id only, so a new account gets back in. What they wrote stays.`
          }
          onClose={() => setConfirm(null)}
        >
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setConfirm(null)}>
              Never mind
            </Button>
            <Button
              kind="danger"
              onClick={async () => {
                if (confirm.action === "kick") {
                  await api.kick(activeGuild.id, confirm.userID);
                } else {
                  await api.ban(activeGuild.id, confirm.userID);
                }
                setConfirm(null);
              }}
            >
              {confirm.action === "kick" ? "Kick them" : "Ban them"}
            </Button>
          </div>
        </Sheet>
      )}

      {newChannel && activeGuild && newChannel !== "category" && (
        <Sheet
          title="Create Channel"
          width={360}
          className="sheet-create"
          onClose={closeCreate}
        >
          <span className="field-label">Channel type</span>
          <div className="kind-picker">
            <button
              type="button"
              className="kind"
              data-active={newChannel === "text"}
              onClick={() => setNewChannel("text")}
            >
              <Icon name="hash" size={15} />
              Text
            </button>
            <button
              type="button"
              className="kind"
              data-active={newChannel === "voice"}
              onClick={() => setNewChannel("voice")}
            >
              <Icon name="speaker-high" size={15} />
              Voice
            </button>
          </div>

          <label className="field">
            <span className="field-label">Channel name</span>
            <span className="prefixed" data-kind={newChannel}>
              <span className="prefixed-mark">
                {newChannel === "voice" ? <Icon name="speaker-high" size={14} /> : "#"}
              </span>
              <input
                className="input"
                value={channelName}
                placeholder="movie-night"
                autoFocus
                onChange={(event) => setChannelName(event.target.value)}
              />
            </span>
          </label>

          <div className="toggle-row">
            <Toggle on={privateChannel} label="Private channel" onChange={setPrivateChannel} />
            <span className="toggle-text">
              <span className="toggle-name">Private channel</span>
              <span className="toggle-desc">Only selected roles and members can view</span>
            </span>
          </div>

          <div className="sheet-actions">
            <Button kind="quiet" onClick={closeCreate}>
              Cancel
            </Button>
            <Button disabled={!channelName.trim()} onClick={() => void makeChannel(activeGuild.id)}>
              Create Channel
            </Button>
          </div>
        </Sheet>
      )}

      {renaming && activeGuild && (
        <Sheet
          title="Change nickname"
          subtitle="It applies in this server only. Clear it to go back to their username."
          onClose={() => setRenaming(null)}
        >
          <label className="field">
            <span className="field-label">Nickname</span>
            <input
              className="input"
              value={renaming.name}
              placeholder={state.names[renaming.userID] ?? ""}
              autoFocus
              onChange={(event) => setRenaming({ ...renaming, name: event.target.value })}
            />
          </label>
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setRenaming(null)}>
              Never mind
            </Button>
            <Button
              onClick={async () => {
                const { userID, name } = renaming;
                setRenaming(null);
                await api.setNickname(activeGuild.id, userID, name.trim() || null);
              }}
            >
              Save it
            </Button>
          </div>
        </Sheet>
      )}

      {editingChannel && (
        <Sheet
          title="Edit channel"
          subtitle="The name is what everybody sees in the list."
          onClose={() => setEditingChannel(null)}
        >
          <label className="field">
            <span className="field-label">Channel name</span>
            <input
              className="input"
              value={editingChannel.name}
              autoFocus
              onChange={(event) =>
                setEditingChannel({ ...editingChannel, name: event.target.value })
              }
            />
          </label>
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setEditingChannel(null)}>
              Never mind
            </Button>
            <Button
              disabled={!editingChannel.name.trim()}
              onClick={async () => {
                const { channel, name } = editingChannel;
                setEditingChannel(null);
                await api.updateChannel(channel.id, { name: name.trim() });
                if (activeGuild) setChannels(await api.channels(activeGuild.id));
              }}
            >
              Rename it
            </Button>
          </div>
        </Sheet>
      )}

      {droppingChannel && (
        <Sheet
          title={`Delete ${droppingChannel.name}`}
          subtitle={
            droppingChannel.kind === "category"
              ? "The channels inside it come loose rather than going with it."
              : "Everything written in it goes too. This cannot be undone."
          }
          onClose={() => setDroppingChannel(null)}
        >
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setDroppingChannel(null)}>
              Never mind
            </Button>
            <Button
              kind="danger"
              onClick={async () => {
                const going = droppingChannel;
                setDroppingChannel(null);
                await api.deleteChannel(going.id);
                if (activeChannel?.id === going.id) setActiveChannel(null);
                if (activeGuild) setChannels(await api.channels(activeGuild.id));
              }}
            >
              Delete it
            </Button>
          </div>
        </Sheet>
      )}

      {newChannel === "category" && activeGuild && (
        <Sheet
          title="Create Category"
          width={340}
          className="sheet-create-category"
          onClose={closeCreate}
        >
          <p className="sheet-note">
            Group related channels together, like Text or Voice.
          </p>
          <label className="field">
            <span className="field-label">Category name</span>
            <input
              className="input"
              value={channelName}
              placeholder="Late night crew"
              autoFocus
              onChange={(event) => setChannelName(event.target.value)}
            />
          </label>
          <div className="sheet-actions">
            <Button kind="quiet" onClick={closeCreate}>
              Cancel
            </Button>
            <Button disabled={!channelName.trim()} onClick={() => void makeChannel(activeGuild.id)}>
              Create Category
            </Button>
          </div>
        </Sheet>
      )}
    </div>
  );
}
