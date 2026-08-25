import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { gateway, type ConnectionState } from "./gateway";
import { Chat } from "./components/Chat";
import { AvatarPicker } from "./components/PickImage";
import { DeleteAccount } from "./components/DeleteAccount";
import { Login } from "./components/Login";
import { MemberList } from "./components/MemberList";
import { Sidebar } from "./components/Sidebar";
import { Voice } from "./components/Voice";
import { emptySession, session, type SessionState } from "./session";
import { serverIsPinned, serverURL } from "./server";
import type { Channel, Guild, GuildRemoval, User } from "./types/events.gen";

function pendingInviteCode(): string | null {
  return new URLSearchParams(location.search).get("invite");
}

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [invite, setInvite] = useState<string | null>(pendingInviteCode);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [removedFrom, setRemovedFrom] = useState<string | null>(null);
  const [booting, setBooting] = useState(true);
  const [connection, setConnection] = useState<ConnectionState>("closed");

  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [activeGuild, setActiveGuild] = useState<Guild | null>(null);
  const [activeChannel, setActiveChannel] = useState<Channel | null>(null);
  const [state, setState] = useState<SessionState>(emptySession);

  useEffect(() => session.onChange(setState), []);

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

  useEffect(() => {
    if (!user) return;
    return gateway.on("GUILD_CREATE", () => {
      setRemovedFrom(null);
      void loadGuilds();
    });
  }, [user, loadGuilds]);

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
      setRemovedFrom(gone.banned ? "You were banned from that server." : "You were removed from that server.");
    });
  }, [user]);

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

  function endSession() {
    gateway.close();
    session.forget();
    setUser(null);
    setGuilds([]);
    setChannels([]);
    setActiveGuild(null);
    setActiveChannel(null);
    setRemovedFrom(null);
  }

  async function logout() {
    await api.logout();
    endSession();
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
        unread={state.unread}
      />

      <main className="main">
        {activeChannel ? (
          activeChannel.kind === "voice" ? (
            <Voice channel={activeChannel} selfID={user.id} />
          ) : (
            <Chat channel={activeChannel} selfID={user.id} />
          )
        ) : (
          <div className="empty">
            {guilds.length === 0
              ? "Create a server with +, or join one with a server id."
              : "Pick a channel."}
          </div>
        )}
      </main>

      <MemberList guild={activeGuild} selfID={user.id} />

      {(inviteError || removedFrom) && (
        <div className="banners">
          {inviteError && <div className="error">{inviteError}</div>}
          {removedFrom && (
            <div className="error">
              {removedFrom}
              <button className="link" onClick={() => setRemovedFrom(null)}>
                Dismiss
              </button>
            </div>
          )}
        </div>
      )}

      <footer className="statusbar">
        <span className={`dot ${connection}`} />
        <span className="muted">
          {connection === "ready"
            ? "connected"
            : connection === "reconnecting"
              ? "reconnecting…"
              : connection}
        </span>
        {serverIsPinned() && (
          <span className="muted server-badge" title="This app is pointed at a non-default server">
            {serverURL()}
          </span>
        )}
        <span className="spacer" />
        <AvatarPicker
          name={user.username}
          imageKey={user.avatar_key}
          onChosen={(key) => setUser({ ...user, avatar_key: key ?? undefined })}
        />
        <span className="muted">{user.username}</span>
        <button className="link" onClick={() => void logout()}>
          Log out
        </button>
        <DeleteAccount onDeleted={endSession} />
      </footer>
    </div>
  );
}
