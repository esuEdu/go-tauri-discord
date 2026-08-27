import { apiBase, serverURL } from "./server";
import type { Channel, Guild, Message, Overwrite, Role, User } from "./types/events.gen";

export type Progress = (fraction: number) => void;

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface Invite {
  code: string;
  guild_id: string;
  guild_name: string;
  inviter_id: string;
  member_count: number;
  uses: number;
  max_uses: number | null;
  expires_at: string | null;
  created_at: string;
}

export interface GuildMember {
  user_id: string;
  username: string;
  nickname: string | null;
  avatar_key: string | null;
  online: boolean;
}

export interface Ban {
  guild_id: string;
  user_id: string;
  username: string;
  banned_by: string | null;
  reason: string | null;
  created_at: string;
}

export interface AuthResponse {
  user: User;
  tokens: TokenPair;
}

function reactionPath(messageID: string, emoji: string): string {
  return `/api/v1/messages/${messageID}/reactions/${encodeURIComponent(emoji)}`;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

export class Api {
  private accessToken: string | null = null;
  private refreshToken: string | null = null;

  constructor() {
    this.accessToken = localStorage.getItem("access_token");
    this.refreshToken = localStorage.getItem("refresh_token");
  }

  get token(): string | null {
    return this.accessToken;
  }

  get authenticated(): boolean {
    return this.accessToken !== null;
  }

  private store(tokens: TokenPair) {
    this.accessToken = tokens.access_token;
    this.refreshToken = tokens.refresh_token;
    localStorage.setItem("access_token", tokens.access_token);
    localStorage.setItem("refresh_token", tokens.refresh_token);
  }

  clear() {
    this.accessToken = null;
    this.refreshToken = null;
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    retryOn401 = true,
  ): Promise<T> {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (this.accessToken) headers["Authorization"] = `Bearer ${this.accessToken}`;

    let res: Response;
    try {
      res = await fetch(apiBase() + path, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new ApiError(0, `could not reach the server at ${serverURL()}`);
    }

    if (res.status === 401 && retryOn401 && this.refreshToken) {
      if (await this.tryRefresh()) {
        return this.request<T>(method, path, body, false);
      }
    }

    if (!res.ok) {
      let message = res.statusText;
      try {
        const parsed = await res.json();
        if (parsed?.error) message = parsed.error;
      } catch {}
      throw new ApiError(res.status, message);
    }

    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  private async tryRefresh(): Promise<boolean> {
    try {
      const res = await fetch(`${apiBase()}/api/v1/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: this.refreshToken }),
      });
      if (!res.ok) {
        this.clear();
        return false;
      }
      this.store(await res.json());
      return true;
    } catch {
      return false;
    }
  }

  async register(username: string, email: string, password: string): Promise<User> {
    const out = await this.request<AuthResponse>("POST", "/api/v1/auth/register", {
      username,
      email,
      password,
    });
    this.store(out.tokens);
    return out.user;
  }

  async login(email: string, password: string): Promise<User> {
    const out = await this.request<AuthResponse>("POST", "/api/v1/auth/login", {
      email,
      password,
    });
    this.store(out.tokens);
    return out.user;
  }

  async logout(): Promise<void> {
    if (this.refreshToken) {
      await this.request("POST", "/api/v1/auth/logout", {
        refresh_token: this.refreshToken,
      }).catch(() => undefined);
    }
    this.clear();
  }

  me(): Promise<User> {
    return this.request<User>("GET", "/api/v1/users/@me");
  }

  async deleteAccount(password: string): Promise<void> {
    await this.request<void>("DELETE", "/api/v1/users/@me", { password });
    this.clear();
  }

  guilds(): Promise<Guild[]> {
    return this.request<Guild[]>("GET", "/api/v1/guilds");
  }

  createGuild(name: string): Promise<Guild> {
    return this.request<Guild>("POST", "/api/v1/guilds", { name });
  }

  createInvite(guildID: string, maxUses?: number, expiresInHours?: number): Promise<Invite> {
    return this.request<Invite>("POST", `/api/v1/guilds/${guildID}/invites`, {
      max_uses: maxUses ?? null,
      expires_in_hours: expiresInHours ?? null,
    });
  }

  invites(guildID: string): Promise<Invite[]> {
    return this.request<Invite[]>("GET", `/api/v1/guilds/${guildID}/invites`);
  }

  revokeInvite(code: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/invites/${code}`);
  }

  previewInvite(code: string): Promise<Invite> {
    return this.request<Invite>("GET", `/api/v1/invites/${code}`);
  }

  redeemInvite(code: string): Promise<Guild> {
    return this.request<Guild>("POST", `/api/v1/invites/${code}`);
  }

  createChannel(
    guildID: string,
    name: string,
    kind: "text" | "voice" | "category",
    position: number,
    parentID?: string,
  ): Promise<Channel> {
    return this.request<Channel>("POST", `/api/v1/guilds/${guildID}/channels`, {
      name,
      kind,
      position,
      parent_id: parentID ?? null,
    });
  }

  roles(guildID: string): Promise<Role[]> {
    return this.request<Role[]>("GET", `/api/v1/guilds/${guildID}/roles`);
  }

  createRole(guildID: string, name: string, permissions: number): Promise<Role> {
    return this.request<Role>("POST", `/api/v1/guilds/${guildID}/roles`, {
      name,
      permissions,
      position: null,
    });
  }

  updateRole(
    roleID: string,
    patch: { name?: string; permissions?: number; position?: number },
  ): Promise<Role> {
    return this.request<Role>("PATCH", `/api/v1/roles/${roleID}`, {
      name: patch.name ?? null,
      permissions: patch.permissions ?? null,
      position: patch.position ?? null,
    });
  }

  deleteRole(roleID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/roles/${roleID}`);
  }

  private send<T>(
    method: string,
    path: string,
    body: XMLHttpRequestBodyInit,
    contentType: string | null,
    onProgress?: Progress,
  ): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const request = new XMLHttpRequest();
      request.open(method, apiBase() + path);
      if (contentType) request.setRequestHeader("Content-Type", contentType);
      if (this.accessToken) {
        request.setRequestHeader("Authorization", `Bearer ${this.accessToken}`);
      }

      if (onProgress) {
        onProgress(0);
        request.upload.onprogress = (event) => {
          if (!event.lengthComputable || event.total === 0) return;
          onProgress(Math.min(1, event.loaded / event.total));
        };
      }

      request.onload = () => {
        let parsed: unknown = undefined;
        try {
          parsed = JSON.parse(request.responseText);
        } catch {}

        if (request.status < 200 || request.status >= 300) {
          const said = (parsed as { error?: string } | undefined)?.error;
          reject(new ApiError(request.status, said ?? request.statusText));
          return;
        }
        resolve(parsed as T);
      };
      request.onerror = () => reject(new ApiError(0, "could not reach the server"));
      request.onabort = () => reject(new ApiError(0, "the upload was cancelled"));

      request.send(body);
    });
  }

  private upload<T>(path: string, file: File, onProgress?: Progress): Promise<T> {
    return this.send<T>("PUT", path, file, file.type, onProgress);
  }

  setAvatar(file: File, onProgress?: Progress): Promise<{ avatar_key: string }> {
    return this.upload<{ avatar_key: string }>("/api/v1/users/@me/avatar", file, onProgress);
  }

  clearAvatar(): Promise<void> {
    return this.request<void>("DELETE", "/api/v1/users/@me/avatar");
  }

  setGuildIcon(guildID: string, file: File, onProgress?: Progress): Promise<{ icon_key: string }> {
    return this.upload<{ icon_key: string }>(`/api/v1/guilds/${guildID}/icon`, file, onProgress);
  }

  clearGuildIcon(guildID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/guilds/${guildID}/icon`);
  }

  moveChannel(channelID: string, position: number): Promise<Channel[]> {
    return this.request<Channel[]>("PATCH", `/api/v1/channels/${channelID}/position`, {
      position,
    });
  }

  members(guildID: string): Promise<GuildMember[]> {
    return this.request<GuildMember[]>("GET", `/api/v1/guilds/${guildID}/members`);
  }

  kick(guildID: string, userID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/guilds/${guildID}/members/${userID}`);
  }

  ban(guildID: string, userID: string, reason?: string): Promise<Ban> {
    return this.request<Ban>("PUT", `/api/v1/guilds/${guildID}/bans/${userID}`, {
      reason: reason?.trim() || null,
    });
  }

  unban(guildID: string, userID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/guilds/${guildID}/bans/${userID}`);
  }

  bans(guildID: string): Promise<Ban[]> {
    return this.request<Ban[]>("GET", `/api/v1/guilds/${guildID}/bans`);
  }

  memberRoles(guildID: string, userID: string): Promise<Role[]> {
    return this.request<Role[]>("GET", `/api/v1/guilds/${guildID}/members/${userID}/roles`);
  }

  assignRole(guildID: string, userID: string, roleID: string): Promise<void> {
    return this.request<void>(
      "PUT",
      `/api/v1/guilds/${guildID}/members/${userID}/roles/${roleID}`,
    );
  }

  unassignRole(guildID: string, userID: string, roleID: string): Promise<void> {
    return this.request<void>(
      "DELETE",
      `/api/v1/guilds/${guildID}/members/${userID}/roles/${roleID}`,
    );
  }

  overwrites(channelID: string): Promise<Overwrite[]> {
    return this.request<Overwrite[]>("GET", `/api/v1/channels/${channelID}/overwrites`);
  }

  setOverwrite(channelID: string, targetID: string, allow: number, deny: number): Promise<void> {
    return this.request<void>("PUT", `/api/v1/channels/${channelID}/overwrites/${targetID}`, {
      target_type: "role",
      allow,
      deny,
    });
  }

  clearOverwrite(channelID: string, targetID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/channels/${channelID}/overwrites/${targetID}`);
  }

  channels(guildID: string): Promise<Channel[]> {
    return this.request<Channel[]>("GET", `/api/v1/guilds/${guildID}/channels`);
  }

  messages(channelID: string, before?: string, limit = 50): Promise<Message[]> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set("before", before);
    return this.request<Message[]>(
      "GET",
      `/api/v1/channels/${channelID}/messages?${params}`,
    );
  }

  sendMessageWithFiles(
    channelID: string,
    content: string,
    files: File[],
    replyTo?: string,
    onProgress?: Progress,
  ): Promise<Message> {
    const form = new FormData();
    if (content) form.append("content", content);
    if (replyTo) form.append("reply_to", replyTo);
    for (const file of files) form.append("files", file, file.name);

    return this.send<Message>(
      "POST",
      `/api/v1/channels/${channelID}/messages`,
      form,
      null,
      onProgress,
    );
  }

  sendMessage(channelID: string, content: string, replyTo?: string): Promise<Message> {
    return this.request<Message>("POST", `/api/v1/channels/${channelID}/messages`, {
      content,
      reply_to: replyTo ?? "",
    });
  }

  editMessage(messageID: string, content: string): Promise<Message> {
    return this.request<Message>("PATCH", `/api/v1/messages/${messageID}`, { content });
  }

  typing(channelID: string): Promise<void> {
    return this.request<void>("POST", `/api/v1/channels/${channelID}/typing`);
  }

  deleteMessage(messageID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/messages/${messageID}`);
  }

  react(messageID: string, emoji: string): Promise<void> {
    return this.request<void>("PUT", reactionPath(messageID, emoji));
  }

  unreact(messageID: string, emoji: string): Promise<void> {
    return this.request<void>("DELETE", reactionPath(messageID, emoji));
  }

  reactors(messageID: string, emoji: string): Promise<User[]> {
    return this.request<User[]>("GET", reactionPath(messageID, emoji));
  }

  markRead(channelID: string, messageID: string): Promise<void> {
    return this.request<void>("PUT", `/api/v1/channels/${channelID}/read`, {
      message_id: messageID,
    });
  }
}

export const api = new Api();
