import { useCallback, useEffect, useRef, useState } from "react";
import { Attachments } from "./Attachments";
import { Avatar } from "./Avatar";
import { Reactions } from "./Reactions";
import { api } from "../api";
import { gateway } from "../gateway";
import { emptySession, session, type SessionState } from "../session";
import { ADD_REACTIONS, SEND_MESSAGES, allows } from "../permissions";
import type {
  Channel,
  Message,
  MessageReaction,
  Reaction,
  TypingStart,
} from "../types/events.gen";

const MAX_FILES = 10;

function withReaction(on: Reaction[], emoji: string, mine: boolean): Reaction[] {
  if (on.some((r) => r.emoji === emoji)) {
    return on.map((r) =>
      r.emoji === emoji ? { ...r, count: r.count + 1, mine: r.mine || mine } : r,
    );
  }
  return [...on, { emoji, count: 1, mine }];
}

function withoutReaction(on: Reaction[], emoji: string, mine: boolean): Reaction[] {
  return on
    .map((r) =>
      r.emoji === emoji ? { ...r, count: r.count - 1, mine: r.mine && !mine } : r,
    )
    .filter((r) => r.count > 0);
}

const PAGE_SIZE = 50;
const TYPING_EVERY = 5000;
const TYPING_FORGET = 8000;

export function Chat({ channel, selfID }: { channel: Channel; selfID: string }) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(true);
  const [draft, setDraft] = useState("");
  const [staged, setStaged] = useState<File[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState("");
  const [typists, setTypists] = useState<string[]>([]);
  const [people, setPeople] = useState<SessionState>(emptySession);

  const maySend = allows(people.channelAllows[channel.id], SEND_MESSAGES);
  const mayReact = allows(people.channelAllows[channel.id], ADD_REACTIONS);

  const scroller = useRef<HTMLDivElement>(null);
  const pinnedToBottom = useRef(true);
  const announcedTyping = useRef(0);
  const filePicker = useRef<HTMLInputElement>(null);

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
      setMessages((prev) =>
        prev.map((m) => (m.id === msg.id ? { ...msg, reactions: m.reactions } : m)),
      );
    });

    const offReact = gateway.on("MESSAGE_REACTION_ADD", (payload) => {
      const hit = payload as MessageReaction;
      if (hit.channel_id !== channel.id) return;
      setMessages((prev) =>
        prev.map((m) =>
          m.id === hit.message_id
            ? { ...m, reactions: withReaction(m.reactions, hit.emoji, hit.user_id === selfID) }
            : m,
        ),
      );
    });

    const offUnreact = gateway.on("MESSAGE_REACTION_REMOVE", (payload) => {
      const hit = payload as MessageReaction;
      if (hit.channel_id !== channel.id) return;
      setMessages((prev) =>
        prev.map((m) =>
          m.id === hit.message_id
            ? { ...m, reactions: withoutReaction(m.reactions, hit.emoji, hit.user_id === selfID) }
            : m,
        ),
      );
    });

    const offDelete = gateway.on("MESSAGE_DELETE", (payload) => {
      const { id, channel_id } = payload as { id: string; channel_id: string };
      if (channel_id !== channel.id) return;
      setMessages((prev) => prev.filter((m) => m.id !== id));
    });

    const offTyping = gateway.on("TYPING_START", (payload) => {
      const start = payload as TypingStart;
      if (start.channel_id !== channel.id || start.user_id === selfID) return;
      setTypists((prev) => (prev.includes(start.user_id) ? prev : [...prev, start.user_id]));
      setTimeout(() => {
        setTypists((prev) => prev.filter((id) => id !== start.user_id));
      }, TYPING_FORGET);
    });

    return () => {
      offCreate();
      offUpdate();
      offDelete();
      offReact();
      offUnreact();
      offTyping();
    };
  }, [channel.id, selfID]);

  useEffect(() => session.onChange(setPeople), []);

  useEffect(() => {
    setTypists([]);
    setEditing(null);
  }, [channel.id]);

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

  function onDraftChange(value: string) {
    setDraft(value);
    if (!value.trim()) return;

    const now = Date.now();
    if (now - announcedTyping.current < TYPING_EVERY) return;
    announcedTyping.current = now;
    void api.typing(channel.id).catch(() => undefined);
  }

  async function saveEdit(e: React.FormEvent) {
    e.preventDefault();
    const id = editing;
    const content = editDraft.trim();
    if (!id || !content) return;

    setEditing(null);
    try {
      await api.editMessage(id, content);
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not edit");
    }
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    const content = draft.trim();
    const files = staged;
    if (!content && files.length === 0) return;

    setDraft("");
    setStaged([]);
    announcedTyping.current = 0;
    pinnedToBottom.current = true;
    try {
      const sent =
        files.length > 0
          ? await api.sendMessageWithFiles(channel.id, content, files)
          : await api.sendMessage(channel.id, content);
      setMessages((prev) =>
        prev.some((m) => m.id === sent.id) ? prev : [...prev, sent],
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not send");
      setDraft(content);
      setStaged(files);
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
                  <Avatar name={m.author.username} imageKey={m.author.avatar_key} />
                  <span className="author">{m.author.username}</span>
                  <span className="muted timestamp">
                    {new Date(m.created_at).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </span>
                </div>
              )}
              {editing === m.id ? (
                <form className="edit-form" onSubmit={saveEdit}>
                  <input
                    value={editDraft}
                    autoFocus
                    onChange={(e) => setEditDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Escape") setEditing(null);
                    }}
                  />
                  <button type="submit" disabled={!editDraft.trim()}>
                    Save
                  </button>
                  <button type="button" className="link" onClick={() => setEditing(null)}>
                    Cancel
                  </button>
                </form>
              ) : (
                <div className="message-body">
                  {m.content}
                  {m.edited_at && <span className="muted"> (edited)</span>}
                  <Attachments attachments={m.attachments} />
                  {m.author.id === selfID && (
                    <>
                      <button
                        className="link edit"
                        title="Edit"
                        onClick={() => {
                          setEditing(m.id);
                          setEditDraft(m.content);
                        }}
                      >
                        ✎
                      </button>
                      <button
                        className="link delete"
                        title="Delete"
                        onClick={() => void api.deleteMessage(m.id).catch(() => undefined)}
                      >
                        ×
                      </button>
                    </>
                  )}
                </div>
              )}
              <Reactions messageID={m.id} reactions={m.reactions} mayReact={mayReact} />
            </div>
          );
        })}
      </div>

      {error && <div className="error inline">{error}</div>}

      {typists.length > 0 && (
        <div className="muted typing">
          {typists.map((id) => session.nameOf(id)).join(", ")}
          {typists.length === 1 ? " is typing…" : " are typing…"}
        </div>
      )}

      {staged.length > 0 && (
        <div className="staged">
          {staged.map((file, i) => (
            <span key={`${file.name}-${i}`} className="staged-file">
              {file.name}
              <button
                type="button"
                className="link"
                aria-label={`Remove ${file.name}`}
                onClick={() => setStaged(staged.filter((_, at) => at !== i))}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}

      <form className="composer" onSubmit={send}>
        <button
          type="button"
          className="secondary"
          disabled={!maySend}
          title="Attach files"
          onClick={() => filePicker.current?.click()}
        >
          +
        </button>
        <input
          ref={filePicker}
          type="file"
          multiple
          hidden
          onChange={(e) => {
            setStaged([...staged, ...Array.from(e.target.files ?? [])].slice(0, MAX_FILES));
            e.target.value = "";
          }}
        />
        <input
          placeholder={
            maySend ? `Message #${channel.name}` : "You cannot post in this channel"
          }
          value={draft}
          disabled={!maySend}
          onChange={(e) => onDraftChange(e.target.value)}
        />
        <button type="submit" disabled={!maySend || (!draft.trim() && staged.length === 0)}>
          Send
        </button>
      </form>
    </div>
  );
}
