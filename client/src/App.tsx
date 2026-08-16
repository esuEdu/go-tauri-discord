import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { gateway, type ConnectionState } from "./gateway";
import { Chat } from "./components/Chat";
import { Login } from "./components/Login";
import { Sidebar } from "./components/Sidebar";
import type { Channel, Guild, User } from "./types/events.gen";

function pendingInviteCode(): string | null {
  return new URLSearchParams(location.search).get("invite");
}

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [invite, setInvite] = useState<string | null>(pendingInviteCode);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [booting, setBooting] = useState(true);
  const [connection, setConnection] = useState<ConnectionState>("closed");

  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [activeGuild, setActiveGuild] = useState<Guild | null>(null);
  const [activeChannel, setActiveChannel] = useState<Channel | null>(null);

  // A stored token may have expired while the app was closed, so the session
  // is only real once /users/@me confirms it.
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
    const off = gateway.onStateChange(setConnection);
    gateway.connect(api.token!);
    return () => {
      off();
      gateway.close();
    };
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
    if (!user || !invite) return;
    let cancelled = false;

    api
      .redeemInvite(invite)
      .then((guild) => {
        if (cancelled) return;
        setActiveGuild(guild);
        void loadGuilds();
      })
      .catch((err) => {
        if (!cancelled) setInviteError(err.message);
      })
      .finally(() => {
        if (cancelled) return;
        setInvite(null);
        const url = new URL(location.href);
        url.searchParams.delete("invite");
        history.replaceState({}, "", url);
      });

    return () => {
      cancelled = true;
    };
  }, [user, invite, loadGuilds]);

  useEffect(() => {
    if (!user) return;
    void loadGuilds();
  }, [user, loadGuilds]);

  // A guild created on another device shows up without a manual refresh.
  useEffect(() => {
    if (!user) return;
    return gateway.on("GUILD_CREATE", () => void loadGuilds());
  }, [user, loadGuilds]);

  useEffect(() => {
    if (!activeGuild) {
      setChannels([]);
      setActiveChannel(null);
      return;
    }
    let cancelled = false;
    api.channels(activeGuild.id).then((list) => {
      if (cancelled) return;
      setChannels(list);
      setActiveChannel(list.find((c) => c.kind === "text") ?? list[0] ?? null);
    });
    return () => {
      cancelled = true;
    };
  }, [activeGuild]);

  useEffect(() => {
    if (!activeGuild) return;
    return gateway.on("CHANNEL_CREATE", (payload) => {
      const channel = payload as Channel;
      if (channel.guild_id !== activeGuild.id) return;
      setChannels((prev) =>
        prev.some((c) => c.id === channel.id) ? prev : [...prev, channel],
      );
    });
  }, [activeGuild]);

  async function logout() {
    await api.logout();
    gateway.close();
    setUser(null);
    setGuilds([]);
    setChannels([]);
    setActiveGuild(null);
    setActiveChannel(null);
  }

  if (booting) return <div className="boot">Loading…</div>;
  if (!user) return <Login onAuthenticated={setUser} inviteCode={invite} />;

  return (
    <div className="app">
      <Sidebar
        guilds={guilds}
        channels={channels}
        activeGuild={activeGuild}
        activeChannel={activeChannel}
        onSelectGuild={setActiveGuild}
        onSelectChannel={setActiveChannel}
        onGuildsChanged={() => void loadGuilds()}
      />

      <main className="main">
        {activeChannel ? (
          <Chat channel={activeChannel} selfID={user.id} />
        ) : (
          <div className="empty">
            {guilds.length === 0
              ? "Create a server with +, or join one with a server id."
              : "Pick a channel."}
          </div>
        )}
      </main>

      {inviteError && <div className="error banner">{inviteError}</div>}

      <footer className="statusbar">
        <span className={`dot ${connection}`} />
        <span className="muted">
          {connection === "ready"
            ? "connected"
            : connection === "reconnecting"
              ? "reconnecting…"
              : connection}
        </span>
        <span className="spacer" />
        <span className="muted">{user.username}</span>
        <button className="link" onClick={() => void logout()}>
          Log out
        </button>
      </footer>
    </div>
  );
}
