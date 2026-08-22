import { api } from "./api";
import { gateway } from "./gateway";
import type { GuildPermissions, Message, PresenceUpdate, Ready } from "./types/events.gen";

export type SessionState = {
  names: Record<string, string>;
  online: Record<string, boolean>;
  unread: Record<string, boolean>;
  guildAllows: Record<string, number>;
  channelAllows: Record<string, number>;
};

export const emptySession: SessionState = {
  names: {},
  online: {},
  unread: {},
  guildAllows: {},
  channelAllows: {},
};

class SessionStore {
  private names: Record<string, string> = {};
  private online: Record<string, boolean> = {};
  private newest: Record<string, string> = {};
  private seen: Record<string, string> = {};
  private open: string | null = null;
  private guildAllows: Record<string, number> = {};
  private channelAllows: Record<string, number> = {};
  private listeners = new Set<(s: SessionState) => void>();

  constructor() {
    gateway.on("READY", (payload) => this.absorb(payload as Ready));
    gateway.on("PRESENCE_UPDATE", (payload) => this.presence(payload as PresenceUpdate));
    gateway.on("MESSAGE_CREATE", (payload) => this.arrived(payload as Message));
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
    this.online = {};
    this.newest = {};
    this.seen = {};
    this.open = null;
    this.guildAllows = {};
    this.channelAllows = {};
    this.emit();
  }

  private snapshot(): SessionState {
    const unread: Record<string, boolean> = {};
    for (const [channelID, newest] of Object.entries(this.newest)) {
      unread[channelID] = (this.seen[channelID] ?? "") < newest;
    }
    return {
      names: this.names,
      online: this.online,
      unread,
      guildAllows: this.guildAllows,
      channelAllows: this.channelAllows,
    };
  }

  private emit() {
    const state = this.snapshot();
    for (const fn of this.listeners) fn(state);
  }

  private absorb(ready: Ready) {
    this.names = { [ready.user.id]: ready.user.username };
    for (const member of ready.members) {
      this.names[member.user.id] = member.user.username;
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
