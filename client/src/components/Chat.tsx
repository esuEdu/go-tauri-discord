import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api";
import { gateway } from "../gateway";
import type { Channel, Message } from "../types/events.gen";

const PAGE_SIZE = 50;

export function Chat({ channel, selfID }: { channel: Channel; selfID: string }) {
  // Stored oldest-first, which is the order they render.
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(true);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);

  const scroller = useRef<HTMLDivElement>(null);
  const pinnedToBottom = useRef(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setMessages([]);
    setHasMore(true);

    api
      .messages(channel.id, undefined, PAGE_SIZE)
      .then((page) => {
        if (cancelled) return;
        setMessages(page.slice().reverse());
        setHasMore(page.length === PAGE_SIZE);
        pinnedToBottom.current = true;
      })
      .catch((err) => !cancelled && setError(err.message))
      .finally(() => !cancelled && setLoading(false));

    return () => {
      cancelled = true;
    };
  }, [channel.id]);

  // Live events. Guild-wide, so filter to the channel on screen.
  useEffect(() => {
    const offCreate = gateway.on("MESSAGE_CREATE", (payload) => {
      const msg = payload as Message;
      if (msg.channel_id !== channel.id) return;
      setMessages((prev) =>
        prev.some((m) => m.id === msg.id) ? prev : [...prev, msg],
      );
    });

    const offUpdate = gateway.on("MESSAGE_UPDATE", (payload) => {
      const msg = payload as Message;
      if (msg.channel_id !== channel.id) return;
      setMessages((prev) => prev.map((m) => (m.id === msg.id ? msg : m)));
    });

    const offDelete = gateway.on("MESSAGE_DELETE", (payload) => {
      const { id, channel_id } = payload as { id: string; channel_id: string };
      if (channel_id !== channel.id) return;
      setMessages((prev) => prev.filter((m) => m.id !== id));
    });

    return () => {
      offCreate();
      offUpdate();
      offDelete();
    };
  }, [channel.id]);

  // Only autoscroll when already at the bottom, so reading history is not
  // yanked away by someone else's message arriving.
  useEffect(() => {
    if (pinnedToBottom.current) {
      scroller.current?.scrollTo({ top: scroller.current.scrollHeight });
    }
  }, [messages]);

  const loadOlder = useCallback(async () => {
    const oldest = messages[0];
    if (!oldest || !hasMore) return;

    const el = scroller.current;
    const before = el?.scrollHeight ?? 0;

    const page = await api.messages(channel.id, oldest.id, PAGE_SIZE);
    setMessages((prev) => [...page.slice().reverse(), ...prev]);
    setHasMore(page.length === PAGE_SIZE);

    // Keep the viewport on the message the reader was looking at.
    requestAnimationFrame(() => {
      if (el) el.scrollTop = el.scrollHeight - before;
    });
  }, [channel.id, messages, hasMore]);

  function onScroll() {
    const el = scroller.current;
    if (!el) return;
    pinnedToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    if (el.scrollTop < 80 && hasMore && !loading) void loadOlder();
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    const content = draft.trim();
    if (!content) return;

    setDraft("");
    pinnedToBottom.current = true;
    try {
      // The gateway echoes this back; MESSAGE_CREATE dedupes on id.
      const sent = await api.sendMessage(channel.id, content);
      setMessages((prev) =>
        prev.some((m) => m.id === sent.id) ? prev : [...prev, sent],
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not send");
      setDraft(content);
    }
  }

  return (
    <div className="chat">
      <header className="chat-header">
        <strong># {channel.name}</strong>
        {channel.topic && <span className="muted"> — {channel.topic}</span>}
      </header>

      <div className="messages" ref={scroller} onScroll={onScroll}>
        {loading && <div className="empty">Loading…</div>}
        {!loading && !hasMore && messages.length === 0 && (
          <div className="empty">No messages yet. Say something.</div>
        )}
        {!loading && hasMore && (
          <button className="link load-more" onClick={() => void loadOlder()}>
            Load older messages
          </button>
        )}

        {messages.map((m, i) => {
          const prev = messages[i - 1];
          const grouped = prev && prev.author.id === m.author.id;
          return (
            <div key={m.id} className={grouped ? "message grouped" : "message"}>
              {!grouped && (
                <div className="message-head">
                  <span className="author">{m.author.username}</span>
                  <span className="muted timestamp">
                    {new Date(m.created_at).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </span>
                </div>
              )}
              <div className="message-body">
                {m.content}
                {m.edited_at && <span className="muted"> (edited)</span>}
                {m.author.id === selfID && (
                  <button
                    className="link delete"
                    title="Delete"
                    onClick={() => void api.deleteMessage(m.id).catch(() => undefined)}
                  >
                    ×
                  </button>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {error && <div className="error inline">{error}</div>}

      <form className="composer" onSubmit={send}>
        <input
          placeholder={`Message #${channel.name}`}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <button type="submit" disabled={!draft.trim()}>
          Send
        </button>
      </form>
    </div>
  );
}
