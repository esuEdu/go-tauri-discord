import type { Channel, Guild, Message, User } from "./types/events.gen";

const BASE = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface AuthResponse {
  user: User;
  tokens: TokenPair;
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

    const res = await fetch(BASE + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });

    // Access tokens are short lived, so one transparent refresh-and-retry
    // keeps the UI from bouncing the user to the login screen every 15 minutes.
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
      } catch {
        // Body was not JSON; the status text is the best we have.
      }
      throw new ApiError(res.status, message);
    }

    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  private async tryRefresh(): Promise<boolean> {
    try {
      const res = await fetch(`${BASE}/api/v1/auth/refresh`, {
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

  guilds(): Promise<Guild[]> {
    return this.request<Guild[]>("GET", "/api/v1/guilds");
  }

  createGuild(name: string): Promise<Guild> {
    return this.request<Guild>("POST", "/api/v1/guilds", { name });
  }

  // Joining by raw guild id is a stand-in until invites land (issue #2).
  joinGuild(guildID: string): Promise<void> {
    return this.request<void>("PUT", `/api/v1/guilds/${guildID}/members/@me`);
  }

  channels(guildID: string): Promise<Channel[]> {
    return this.request<Channel[]>("GET", `/api/v1/guilds/${guildID}/channels`);
  }

  // Returns newest first. Pass the oldest id you hold as `before` to page back.
  messages(channelID: string, before?: string, limit = 50): Promise<Message[]> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (before) params.set("before", before);
    return this.request<Message[]>(
      "GET",
      `/api/v1/channels/${channelID}/messages?${params}`,
    );
  }

  sendMessage(channelID: string, content: string): Promise<Message> {
    return this.request<Message>("POST", `/api/v1/channels/${channelID}/messages`, {
      content,
    });
  }

  deleteMessage(messageID: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/messages/${messageID}`);
  }
}

export const api = new Api();
