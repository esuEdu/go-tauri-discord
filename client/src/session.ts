import { api } from "./api";
import { gateway } from "./gateway";
import type {
  Guild,
  GuildPermissions,
  GuildRemoval,
  Member,
  Message,
  PresenceUpdate,
  Ready,
  VoiceStateUpdate,
} from "./types/events.gen";

export type SessionState = {
  names: Record<string, string>;
  avatars: Record<string, string | null>;
  online: Record<string, boolean>;
  unread: Record<string, boolean>;
  guildAllows: Record<string, number>;
  channelAllows: Record<string, number>;
  membersByGuild: Record<string, string[]>;
  inVoice: Record<string, string[]>;
  mutedInVoice: Record<string, boolean>;
  deafenedInVoice: Record<string, boolean>;
};

export const emptySession: SessionState = {
  names: {},
  avatars: {},
  online: {},
  unread: {},
  guildAllows: {},
  channelAllows: {},
  membersByGuild: {},
  inVoice: {},
  mutedInVoice: {},
  deafenedInVoice: {},
};

class SessionStore {
  private names: Record<string, string> = {};
  private avatars: Record<string, string | null> = {};
  private online: Record<string, boolean> = {};
  private newest: Record<string, string> = {};
  private seen: Record<string, string> = {};
  private open: string | null = null;
  private guildAllows: Record<string, number> = {};
  private channelAllows: Record<string, number> = {};
  private membersByGuild: Record<string, string[]> = {};
  private inVoice: Record<string, string[]> = {};
  private mutedInVoice: Record<string, boolean> = {};
  private deafenedInVoice: Record<string, boolean> = {};
  private listeners = new Set<(s: SessionState) => void>();

  constructor() {
    gateway.on("READY", (payload) => this.absorb(payload as Ready));
    gateway.on("PRESENCE_UPDATE", (payload) => this.presence(payload as PresenceUpdate));
    gateway.on("MESSAGE_CREATE", (payload) => this.arrived(payload as Message));
    gateway.on("GUILD_MEMBER_ADD", (payload) => {
      const member = payload as Member;
      this.names = { ...this.names, [member.user.id]: member.user.username };
      this.avatars = { ...this.avatars, [member.user.id]: member.user.avatar_key ?? null };
      this.remember(member.guild_id, member.user.id);
      this.emit();
    });
    gateway.on("GUILD_CREATE", (payload) => {
      void this.roster((payload as Guild).id);
    });
    gateway.on("GUILD_MEMBER_REMOVE", (payload) => {
      const gone = payload as GuildRemoval;
      this.forgetMember(gone.guild_id, gone.user_id);
      this.emit();
    });
    gateway.on("GUILD_REMOVE", (payload) => {
      this.forgetGuild((payload as GuildRemoval).guild_id);
      this.emit();
    });
    gateway.on("VOICE_STATE_UPDATE", (payload) => {
      this.voiceMoved(payload as VoiceStateUpdate);
      this.emit();
    });
    gateway.on("PERMISSIONS_UPDATE", (payload) => {
      this.absorbAllowed([payload as GuildPermissions]);
      this.emit();
    });
  }

  onChange(fn: (s: SessionState) => void): () => void {
    this.listeners.add(fn);
    fn(this.snapshot());
    return () => this.listeners.delete(fn);
  }

  nameOf(userID: string): string {
    return this.names[userID] ?? userID.slice(0, 8);
  }

  reading(channelID: string | null) {
    this.open = channelID;
    if (channelID) this.catchUp(channelID);
    this.emit();
  }

  forget() {
    this.names = {};
    this.avatars = {};
    this.online = {};
    this.newest = {};
    this.seen = {};
    this.open = null;
    this.guildAllows = {};
    this.channelAllows = {};
    this.membersByGuild = {};
    this.inVoice = {};
    this.mutedInVoice = {};
    this.deafenedInVoice = {};
    this.emit();
  }

  private snapshot(): SessionState {
    const unread: Record<string, boolean> = {};
    for (const [channelID, newest] of Object.entries(this.newest)) {
      unread[channelID] = (this.seen[channelID] ?? "") < newest;
    }
    return {
      names: this.names,
      avatars: this.avatars,
      online: this.online,
      unread,
      guildAllows: this.guildAllows,
      channelAllows: this.channelAllows,
      membersByGuild: this.membersByGuild,
      inVoice: this.inVoice,
      deafenedInVoice: this.deafenedInVoice,
      mutedInVoice: this.mutedInVoice,
    };
  }

  private emit() {
    const state = this.snapshot();
    for (const fn of this.listeners) fn(state);
  }

  private absorb(ready: Ready) {
    this.names = { [ready.user.id]: ready.user.username };
    this.avatars = { [ready.user.id]: ready.user.avatar_key ?? null };
    this.membersByGuild = {};
    this.inVoice = {};
    this.mutedInVoice = {};
    this.deafenedInVoice = {};
    for (const state of ready.voice) this.voiceMoved(state);
    for (const member of ready.members) {
      this.names[member.user.id] = member.user.username;
      this.avatars[member.user.id] = member.user.avatar_key ?? null;
      this.remember(member.guild_id, member.user.id);
    }

    this.online = {};
    for (const id of ready.online) this.online[id] = true;

    this.newest = {};
    for (const channel of ready.channels) {
      if (channel.last_message_id) this.newest[channel.id] = channel.last_message_id;
    }

    this.seen = {};
    for (const state of ready.read_states) {
      if (state.last_read_message_id) this.seen[state.channel_id] = state.last_read_message_id;
    }

    this.guildAllows = {};
    this.channelAllows = {};
    this.absorbAllowed(ready.allowed);

    if (this.open) this.catchUp(this.open);
    this.emit();
  }

  private voiceMoved(state: VoiceStateUpdate) {
    const next: Record<string, string[]> = {};
    for (const [channelID, people] of Object.entries(this.inVoice)) {
      const without = people.filter((id) => id !== state.user_id);
      if (without.length > 0) next[channelID] = without;
    }

    if (state.channel_id) {
      next[state.channel_id] = [...(next[state.channel_id] ?? []), state.user_id];
    }
    this.inVoice = next;
    this.mutedInVoice = {
      ...this.mutedInVoice,
      [state.user_id]: Boolean(state.channel_id) && state.self_mute,
    };
    this.deafenedInVoice = {
      ...this.deafenedInVoice,
      [state.user_id]: Boolean(state.channel_id) && state.self_deaf,
    };
  }

  private remember(guildID: string, userID: string) {
    const here = this.membersByGuild[guildID] ?? [];
    if (here.includes(userID)) return;
    this.membersByGuild = { ...this.membersByGuild, [guildID]: [...here, userID] };
  }

  private async roster(guildID: string) {
    if (this.membersByGuild[guildID]) return;

    const members = await api.members(guildID).catch(() => null);
    if (!members) return;

    const names = { ...this.names };
    const avatars = { ...this.avatars };
    const online = { ...this.online };
    for (const member of members) {
      names[member.user_id] = member.username;
      avatars[member.user_id] = member.avatar_key ?? null;
      online[member.user_id] = member.online;
      this.remember(guildID, member.user_id);
    }
    this.names = names;
    this.avatars = avatars;
    this.online = online;
    this.emit();
  }

  private forgetMember(guildID: string, userID: string) {
    const here = this.membersByGuild[guildID];
    if (!here || !here.includes(userID)) return;
    this.membersByGuild = {
      ...this.membersByGuild,
      [guildID]: here.filter((id) => id !== userID),
    };
  }

  private forgetGuild(guildID: string) {
    const { [guildID]: _members, ...restMembers } = this.membersByGuild;
    const { [guildID]: _allowed, ...restAllows } = this.guildAllows;
    this.membersByGuild = restMembers;
    this.guildAllows = restAllows;
  }

  private absorbAllowed(allowed: GuildPermissions[]) {
    const guilds = { ...this.guildAllows };
    const channels = { ...this.channelAllows };

    for (const guild of allowed) {
      guilds[guild.guild_id] = guild.permissions;
      for (const channel of guild.channels) {
        channels[channel.channel_id] = channel.permissions;
      }
    }
    this.guildAllows = guilds;
    this.channelAllows = channels;
  }

  private presence(update: PresenceUpdate) {
    this.online = { ...this.online, [update.user_id]: update.status === "online" };
    this.emit();
  }

  private arrived(message: Message) {
    this.newest[message.channel_id] = message.id;
    if (message.channel_id === this.open) this.catchUp(message.channel_id);
    this.emit();
  }

  private catchUp(channelID: string) {
    const newest = this.newest[channelID];
    if (!newest || this.seen[channelID] === newest) return;

    this.seen[channelID] = newest;
    void api.markRead(channelID, newest).catch(() => undefined);
  }
}

export const session = new SessionStore();
