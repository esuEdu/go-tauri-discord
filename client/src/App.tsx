import { useCallback, useEffect, useState } from "react";
import { api } from "./api";
import { gateway, type ConnectionState } from "./gateway";
import { Chat } from "./components/Chat";
import { DeleteAccount } from "./components/DeleteAccount";
import { Icon } from "./components/Icon";
import { Login } from "./components/Login";
import { MemberList } from "./components/MemberList";
import { Sidebar } from "./components/Sidebar";
import { UserBar } from "./components/UserBar";
import { Voice } from "./components/Voice";
import { emptySession, session, type SessionState } from "./session";
import type { Channel, Guild, GuildRemoval, User } from "./types/events.gen";

function pendingInviteCode(): string | null {
  return new URLSearchParams(location.search).get("invite");
}

function Booting() {
  return (
    <div className="boot">
      <span className="boot-block" style={{ width: 60, height: "100%" }} />
      <span className="boot-block" style={{ width: 248, height: "100%" }} />
      <span className="boot-block" style={{ flex: 1, height: "100%" }} />
    </div>
  );
}

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [invite, setInvite] = useState<string | null>(pendingInviteCode);
  const [inviteError, setInviteError] = useState<string | null>(null);
  const [removedFrom, setRemovedFrom] = useState<string | null>(null);
  const [booting, setBooting] = useState(true);
  const [connection, setConnection] = useState<ConnectionState>("closed");
  const [leaving, setLeaving] = useState(false);
  const [watching, setWatching] = useState<string | null>(null);

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
        setActiveGuild((current) => (current?.id === gone.guild_id ? (left[0] ?? null) : current));
        return left;
      });
      setRemovedFrom(
        gone.banned ? "You were banned from that server." : "You were removed from that server.",
      );
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

    return () => {
      forgetCreate();
      forgetUpdate();
    };
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
    setLeaving(false);
  }

  async function logout() {
    await api.logout();
    endSession();
  }

  if (booting) return <Booting />;
  if (!user) return <Login onAuthenticated={setUser} inviteCode={invite} />;

  return (
    <div className="app">
      {leaving && <DeleteAccount onDeleted={endSession} onClose={() => setLeaving(false)} />}

      {(inviteError || removedFrom) && (
        <div className="toasts">
          {inviteError && (
            <div className="banner">
              <Icon name="warning-circle" size={16} />
              <span className="grow">{inviteError}</span>
              <span className="banner-actions">
                <button className="link quiet" onClick={() => setInviteError(null)}>
                  Dismiss
                </button>
              </span>
            </div>
          )}
          {removedFrom && (
            <div className="banner">
              <Icon name="warning-circle" size={16} />
              <span className="grow">{removedFrom}</span>
              <span className="banner-actions">
                <button className="link quiet" onClick={() => setRemovedFrom(null)}>
                  Dismiss
                </button>
              </span>
            </div>
          )}
        </div>
      )}

      <div className="left-column">
        <div className="left-column-top">
          <Sidebar
            guilds={guilds}
            channels={channels}
            activeGuild={activeGuild}
            activeChannel={activeChannel}
            selfID={user.id}
            watching={watching}
            onWatch={setWatching}
            onSelectGuild={setActiveGuild}
            onSelectChannel={setActiveChannel}
            onGuildsChanged={() => void loadGuilds()}
            unread={state.unread}
          />
        </div>

        <UserBar
          user={user}
          connection={connection}
          channels={channels}
          onUserChanged={setUser}
          onSignOut={() => void logout()}
          onDeleteAccount={() => setLeaving(true)}
          onOpenChannel={setActiveChannel}
          watching={watching}
        />
      </div>

      <main className="card main">
        {activeChannel ? (
          activeChannel.kind === "voice" ? (
            <Voice
              key={activeChannel.id}
              channel={activeChannel}
              selfID={user.id}
              watching={watching}
              onWatch={setWatching}
            />
          ) : (
            <Chat key={activeChannel.id} channel={activeChannel} selfID={user.id} />
          )
        ) : (
          <div className="room-empty">
            {guilds.length === 0
              ? "Start a server with the plus, or step into one you were given a code for."
              : "Pick a channel."}
          </div>
        )}
      </main>

      {activeChannel?.kind !== "voice" && <MemberList guild={activeGuild} selfID={user.id} />}
    </div>
  );
}
