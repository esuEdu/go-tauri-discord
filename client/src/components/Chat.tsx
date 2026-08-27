import { useCallback, useEffect, useRef, useState } from "react";
import { Attachments } from "./Attachments";
import { Avatar } from "./Avatar";
import { DeleteMessage } from "./DeleteMessage";
import { Icon } from "./Icon";
import { AddReaction, Reactions } from "./Reactions";
import { ReplyQuote } from "./ReplyQuote";
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
const MAX_FILE_BYTES = 25 << 20;
const MAX_CONTENT = 4000;
const PAGE_SIZE = 50;
const TYPING_EVERY = 5000;
const TYPING_FORGET = 8000;

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
    .map((r) => (r.emoji === emoji ? { ...r, count: r.count - 1, mine: r.mine && !mine } : r))
    .filter((r) => r.count > 0);
}

function dayOf(at: string): string {
  return new Date(at).toDateString();
}

function dayName(at: string): string {
  const day = new Date(at);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);

  if (day.toDateString() === today.toDateString()) return "Today";
  if (day.toDateString() === yesterday.toDateString()) return "Yesterday";
  return day.toLocaleDateString([], { day: "numeric", month: "long" });
}

function clockOf(at: string): string {
  return new Date(at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function StagedThumb({ file }: { file: File }) {
  const [preview, setPreview] = useState<string | null>(null);

  useEffect(() => {
    if (!file.type.startsWith("image/")) return;
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  if (preview) return <img className="staged-thumb" src={preview} alt="" />;

  return (
    <span className="staged-thumb">
      <Icon name="file" size={20} />
    </span>
  );
}

function refuse(files: File[]): string | null {
  const heavy = files.find((f) => f.size > MAX_FILE_BYTES);
  if (heavy) {
    return `${heavy.name} is ${(heavy.size / (1 << 20)).toFixed(0)} MB — 25 MB is the most per file.`;
  }
  if (files.length > MAX_FILES) {
    return `A message carries ten files; you picked ${files.length}. The rest are staged, so nothing is lost by trying.`;
  }
  return null;
}

export function Chat({ channel, selfID }: { channel: Channel; selfID: string }) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(true);
  const [draft, setDraft] = useState("");
  const [staged, setStaged] = useState<File[]>([]);
  const [sending, setSending] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState("");
  const [deleting, setDeleting] = useState<Message | null>(null);
  const [typists, setTypists] = useState<string[]>([]);
  const [people, setPeople] = useState<SessionState>(emptySession);
  const [replyingTo, setReplyingTo] = useState<Message | null>(null);
  const [highlighted, setHighlighted] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);

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
      setMessages((prev) => (prev.some((m) => m.id === msg.id) ? prev : [...prev, msg]));
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
      setReplyingTo((prev) => (prev?.id === id ? null : prev));
      setDeleting((prev) => (prev?.id === id ? null : prev));
      setMessages((prev) =>
        prev
          .filter((m) => m.id !== id)
          .map((m) =>
            m.reply_to && m.reply_to.message_id === id
              ? {
                  ...m,
                  reply_to: {
                    ...m.reply_to,
                    deleted: true,
                    content: "",
                    author: undefined,
                    truncated: false,
                    has_attachments: false,
                  },
                }
              : m,
          ),
      );
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
    setReplyingTo(null);
    setHighlighted(null);
    setStaged([]);
    setError(null);
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

  function stage(picked: File[]) {
    if (picked.length === 0) return;
    const refused = refuse(picked);
    setError(refused);
    setStaged((prev) =>
      [...prev, ...picked.filter((f) => f.size <= MAX_FILE_BYTES)].slice(0, MAX_FILES),
    );
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

  function jumpTo(messageID: string) {
    const target = document.getElementById(`message-${messageID}`);
    if (!target) {
      setError("That message is too far back to jump to. Scroll up and load older ones first.");
      return;
    }
    pinnedToBottom.current = false;
    target.scrollIntoView({ block: "center", behavior: "smooth" });
    setHighlighted(messageID);
    setTimeout(() => setHighlighted((at) => (at === messageID ? null : at)), 1600);
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    const content = draft.trim();
    const files = staged;
    if (!content && files.length === 0) return;

    const answering = replyingTo;
    setDraft("");
    setStaged([]);
    setReplyingTo(null);
    setError(null);
    announcedTyping.current = 0;
    pinnedToBottom.current = true;
    try {
      const sent =
        files.length > 0
          ? await api.sendMessageWithFiles(channel.id, content, files, answering?.id, setSending)
          : await api.sendMessage(channel.id, content, answering?.id);
      setMessages((prev) => (prev.some((m) => m.id === sent.id) ? prev : [...prev, sent]));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Not sent — the connection dropped.");
      setDraft(content);
      setStaged(files);
      setReplyingTo(answering);
    } finally {
      setSending(null);
    }
  }

  const left = MAX_CONTENT - [...draft].length;

  return (
    <>
      {deleting && <DeleteMessage message={deleting} onClose={() => setDeleting(null)} />}

      <header className="room-head">
        <span className="room-name">#{channel.name}</span>
        {channel.topic && <span className="room-topic clip">{channel.topic}</span>}
      </header>

      <div className="messages" ref={scroller} onScroll={onScroll}>
        {loading && (
          <div className="skeleton">
            <span />
            <span />
          </div>
        )}

        {!loading && !hasMore && messages.length === 0 && (
          <div className="room-empty">
            This is the beginning of <strong>#{channel.name}</strong>. Say something.
          </div>
        )}

        {!loading && hasMore && (
          <button className="btn btn-primary btn-small load-older" onClick={() => void loadOlder()}>
            <Icon name="arrow-up" size={14} />
            Load what came before
          </button>
        )}

        {messages.map((m, i) => {
          const prev = messages[i - 1];
          const newDay = !prev || dayOf(prev.created_at) !== dayOf(m.created_at);
          const grouped =
            !newDay && prev && prev.author.id === m.author.id && !m.reply_to && !prev.reply_to;

          const classes = ["message"];
          if (grouped) classes.push("grouped");
          if (highlighted === m.id) classes.push("highlighted");

          return (
            <div key={m.id} className="stack tight">
              {newDay && <div className="day-divider">{dayName(m.created_at)}</div>}

              <div id={`message-${m.id}`} className={classes.join(" ")}>
                {grouped ? (
                  <span className="message-gutter" />
                ) : (
                  <Avatar
                    name={m.author.username}
                    imageKey={m.author.avatar_key}
                    mine={m.author.id === selfID}
                  />
                )}

                <div className="message-body">
                  {m.reply_to && <ReplyQuote reply={m.reply_to} onJump={jumpTo} />}

                  {!grouped && (
                    <div className="message-head">
                      <span className="message-author">{session.labelOf(m.author.id)}</span>
                      <span className="message-time">{clockOf(m.created_at)}</span>
                    </div>
                  )}

                  {editing === m.id ? (
                    <form className="row" onSubmit={saveEdit}>
                      <input
                        className="input grow"
                        value={editDraft}
                        autoFocus
                        onChange={(e) => setEditDraft(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Escape") setEditing(null);
                        }}
                      />
                      <button type="submit" className="btn btn-primary btn-small" disabled={!editDraft.trim()}>
                        Save
                      </button>
                      <button
                        type="button"
                        className="btn btn-quiet btn-small"
                        onClick={() => setEditing(null)}
                      >
                        Cancel
                      </button>
                    </form>
                  ) : (
                    <div className="message-text">
                      {m.content}
                      {m.edited_at && <span className="message-edited"> (edited)</span>}
                    </div>
                  )}

                  <Attachments attachments={m.attachments} />
                  <Reactions messageID={m.id} reactions={m.reactions} mayReact={mayReact} />
                </div>

                <div className="message-actions">
                  {mayReact && (
                    <AddReaction
                      messageID={m.id}
                      reactions={m.reactions}
                      className="icon-button"
                    />
                  )}
                  {maySend && (
                    <button
                      className="icon-button"
                      title="Reply"
                      onClick={() => setReplyingTo(m)}
                    >
                      <Icon name="arrow-bend-up-left" size={15} />
                    </button>
                  )}
                  {m.author.id === selfID && (
                    <>
                      <button
                        className="icon-button"
                        title="Edit"
                        onClick={() => {
                          setEditing(m.id);
                          setEditDraft(m.content);
                        }}
                      >
                        <Icon name="pencil-simple" size={15} />
                      </button>
                      <button
                        className="icon-button"
                        title="Delete"
                        onClick={() => setDeleting(m)}
                      >
                        <Icon name="trash" size={15} />
                      </button>
                    </>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      <div
        className="composer-area"
        onDragOver={(e) => {
          if (!maySend) return;
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          if (!maySend) return;
          e.preventDefault();
          setDragging(false);
          stage(Array.from(e.dataTransfer.files));
        }}
      >
        <div className="typing">
          {typists.length > 0 && (
            <>
              <span className="bars quiet">
                <span />
                <span />
                <span />
              </span>
              {typists.map((id) => session.nameOf(id)).join(", ")}
              {typists.length === 1 ? " is typing" : " are typing"}
            </>
          )}
        </div>

        {error && (
          <div className="banner bad">
            <Icon name="warning-circle" size={15} />
            <span className="grow">{error}</span>
            <span className="banner-actions">
              <button className="link quiet" onClick={() => setError(null)}>
                Dismiss
              </button>
            </span>
          </div>
        )}

        {sending !== null && (
          <div className="upload-progress">
            <span>{sending < 1 ? `Sending files… ${Math.round(sending * 100)}%` : "Almost there…"}</span>
            <span className="upload-bar">
              <span style={{ width: `${Math.round(sending * 100)}%` }} />
            </span>
          </div>
        )}

        {dragging && (
          <div className="drop-target">
            <Icon name="tray-arrow-down" size={26} />
            <span className="drop-target-title">Drop to attach to #{channel.name}</span>
            <span className="note">They go with your next message, not before it.</span>
          </div>
        )}

        <form className={maySend ? "composer" : "composer blocked"} onSubmit={send}>
          {replyingTo && (
            <div className="composer-reply">
              <Icon name="arrow-bend-up-left" size={14} />
              Replying to <strong>{session.labelOf(replyingTo.author.id)}</strong>
              <button
                type="button"
                className="composer-reply-close"
                aria-label="Stop replying"
                onClick={() => setReplyingTo(null)}
              >
                Esc
                <Icon name="x" size={12} />
              </button>
            </div>
          )}

          {staged.length > 0 && sending === null && (
            <div className="composer-staged">
              {staged.map((file, i) => (
                <span key={`${file.name}-${i}`} className="staged-file">
                  <StagedThumb file={file} />
                  <span className="staged-name">{file.name}</span>
                  <button
                    type="button"
                    className="staged-drop"
                    aria-label={`Remove ${file.name}`}
                    onClick={() => setStaged(staged.filter((_, at) => at !== i))}
                  >
                    <Icon name="x" size={10} />
                  </button>
                </span>
              ))}
              <span className="staged-count">
                {staged.length} of {MAX_FILES}
              </span>
            </div>
          )}

          <div className="composer-row">
            <button
              type="button"
              className="composer-icon"
              disabled={!maySend}
              title="Attach files"
              onClick={() => filePicker.current?.click()}
            >
              <Icon name={maySend ? "paperclip" : "prohibit"} size={18} />
            </button>

            <input
              ref={filePicker}
              type="file"
              multiple
              hidden
              onChange={(e) => {
                stage(Array.from(e.target.files ?? []));
                e.target.value = "";
              }}
            />

            <input
              className="composer-input"
              placeholder={
                !maySend
                  ? "You can read here, but not post."
                  : replyingTo
                    ? `Reply to ${session.nameOf(replyingTo.author.id)}`
                    : `Message #${channel.name}`
              }
              value={draft}
              disabled={!maySend}
              maxLength={MAX_CONTENT}
              onChange={(e) => onDraftChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Escape") setReplyingTo(null);
              }}
            />

            {draft.length > 0 && <span className="composer-left">{left} left</span>}

            <button
              type="submit"
              className="composer-icon"
              disabled={!maySend || (!draft.trim() && staged.length === 0)}
              title="Send"
            >
              <Icon name="paper-plane-tilt" size={18} />
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
