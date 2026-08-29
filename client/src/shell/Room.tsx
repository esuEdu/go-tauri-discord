import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type MouseEvent,
} from "react";
import { api } from "../api";
import type { Attachment, Channel, Message } from "../types/events.gen";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { IconButton } from "../ui/IconButton";
import { ReactionPill } from "../ui/ReactionPill";

const MAX_CONTENT = 4000;
const MAX_FILES = 10;

export function Room({
  channel,
  messages,
  nameFor,
  meID,
  typing,
  canSend,
  canReact,
  insert,
  avatarURL,
  onReact,
  onUnreact,
  onOpenEmoji,
  onOpenMessageMenu,
  onOpenImage,
  onSent,
}: {
  channel: Channel;
  messages: Message[];
  nameFor: (id: string) => string;
  meID: string;
  typing: string[];
  canSend: boolean;
  canReact: boolean;
  insert: { char: string; n: number } | null;
  avatarURL: (id: string) => string | null;
  onReact: (messageID: string, emoji: string) => void;
  onUnreact: (messageID: string, emoji: string) => void;
  onOpenEmoji: (messageID: string | null, anchor: DOMRect) => void;
  onOpenMessageMenu: (message: Message, event: MouseEvent<HTMLElement>) => void;
  onOpenImage: (attachment: Attachment, message: Message) => void;
  onSent: () => void;
}) {
  const [draft, setDraft] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [replyTo, setReplyTo] = useState<Message | null>(null);
  const [focused, setFocused] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  const [flashing, setFlashing] = useState<string | null>(null);
  const [scrolling, setScrolling] = useState(false);
  const idle = useRef<number | undefined>(undefined);
  const listRef = useRef<HTMLDivElement>(null);
  const rows = useRef(new Map<string, HTMLElement>());
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const picker = useRef<HTMLInputElement>(null);

  useEffect(() => () => window.clearTimeout(idle.current), []);

  useEffect(() => {
    setDraft("");
    setFiles([]);
    setReplyTo(null);
  }, [channel.id]);

  useEffect(() => {
    if (!insert) return;
    setDraft((was) => was + insert.char);
    inputRef.current?.focus();
  }, [insert?.n]);

  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    const toBottom = () => {
      list.scrollTop = list.scrollHeight;
    };
    toBottom();
    const frame = requestAnimationFrame(toBottom);
    const later = window.setTimeout(toBottom, 120);
    return () => {
      cancelAnimationFrame(frame);
      window.clearTimeout(later);
    };
  }, [messages.length, channel.id]);

  function jumpTo(messageID: string) {
    const node = rows.current.get(messageID);
    if (!node) return;
    node.scrollIntoView({ block: "center", behavior: "smooth" });
    setFlashing(messageID);
    window.setTimeout(
      () => setFlashing((was) => (was === messageID ? null : was)),
      1600,
    );
  }

  const groups = useMemo(() => groupMessages(messages), [messages]);
  const left = MAX_CONTENT - [...draft].length;
  const sendable = canSend && (draft.trim().length > 0 || files.length > 0) && left >= 0;

  async function send() {
    if (!sendable || progress !== null) return;
    const content = draft.trim();
    const carried = files;
    setDraft("");
    setFiles([]);
    const parent = replyTo;
    setReplyTo(null);
    try {
      if (carried.length) {
        setProgress(0);
        await api.sendMessageWithFiles(
          channel.id,
          content,
          carried,
          parent?.id,
          setProgress,
        );
      } else {
        await api.sendMessage(channel.id, content, parent?.id);
      }
      onSent();
    } finally {
      setProgress(null);
    }
  }

  function addFiles(incoming: File[]) {
    setFiles((was) => [...was, ...incoming].slice(0, MAX_FILES));
  }

  function onDrop(event: DragEvent) {
    event.preventDefault();
    setDragOver(false);
    addFiles([...event.dataTransfer.files]);
  }

  return (
    <section className="room panel">
      <header className="room-head">
        <span className="room-name">
          {channel.kind === "voice" ? "" : "#"}
          {channel.name}
        </span>
        {channel.topic && <span className="room-topic">{channel.topic}</span>}
      </header>

      <div
        className="messages"
        ref={listRef}
        data-scrolling={scrolling}
        onScroll={() => {
          setScrolling(true);
          window.clearTimeout(idle.current);
          idle.current = window.setTimeout(() => setScrolling(false), 800);
        }}
      >
        {groups.map((group) =>
          group.kind === "day" ? (
            <div className="day-divider" key={group.key}>
              {group.label}
            </div>
          ) : (
            <MessageRow
              key={group.message.id}
              message={group.message}
              grouped={group.grouped}
              highlighted={
                flashing === group.message.id || replyTo?.id === group.message.id
              }
              onMounted={(node) => {
                if (node) rows.current.set(group.message.id, node);
                else rows.current.delete(group.message.id);
              }}
              onJumpTo={jumpTo}
              nameFor={nameFor}
              meID={meID}
              canReact={canReact}
              avatarURL={avatarURL}
              onReact={onReact}
              onUnreact={onUnreact}
              onOpenEmoji={onOpenEmoji}
              onReply={() => {
                setReplyTo(group.message);
                inputRef.current?.focus();
              }}
              onOpenMenu={onOpenMessageMenu}
              onOpenImage={onOpenImage}
            />
          ),
        )}
      </div>

      <div className="composer-area">
        <div className="typing">
          {typing.length > 0 && (
            <>
              <span className="typing-bars">
                <i />
                <i />
                <i />
              </span>
              {sentenceFor(typing)}
            </>
          )}
        </div>

        <div
          className="composer"
          data-focused={focused}
          onDragOver={(event) => {
            event.preventDefault();
            setDragOver(true);
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={onDrop}
        >
          {dragOver && (
            <div className="drop-overlay">
              Drop to attach
              <span>Up to {MAX_FILES} files</span>
            </div>
          )}

          {replyTo && (
            <div className="composer-reply">
              <span className="composer-reply-label">Replying to</span>
              <span className="composer-reply-name">
                {nameFor(replyTo.author.id)}
              </span>
              <button
                type="button"
                className="composer-esc"
                onClick={() => setReplyTo(null)}
              >
                Esc
              </button>
            </div>
          )}

          {files.length > 0 && (
            <div className="composer-attachments">
              {files.map((file, index) => (
                <div className="composer-thumb" key={`${file.name}-${index}`}>
                  {file.type.startsWith("image/") ? (
                    <img
                      className="composer-thumb-picture"
                      src={URL.createObjectURL(file)}
                      alt=""
                    />
                  ) : (
                    <span className="composer-thumb-file">
                      <Icon name="paperclip" size={20} />
                    </span>
                  )}
                  <span className="composer-thumb-name">{file.name}</span>
                  <button
                    type="button"
                    className="composer-thumb-remove"
                    aria-label={`Remove ${file.name}`}
                    onClick={() =>
                      setFiles((was) => was.filter((_, at) => at !== index))
                    }
                  >
                    <Icon name="x" size={10} />
                  </button>
                </div>
              ))}
              <span className="composer-count-of">
                {files.length} of {MAX_FILES}
              </span>
            </div>
          )}

          {progress !== null && (
            <div className="upload-bar">
              <i style={{ width: `${Math.round(progress * 100)}%` }} />
            </div>
          )}

          <div className="composer-row">
            <input
              ref={picker}
              type="file"
              multiple
              hidden
              onChange={(event) => {
                addFiles([...(event.target.files ?? [])]);
                event.target.value = "";
              }}
            />
            <button
              type="button"
              className="composer-icon"
              aria-label="Attach a file"
              title="Attach a file"
              disabled={!canSend}
              onClick={() => picker.current?.click()}
            >
              <Icon name="paperclip" size={18} />
            </button>
            <textarea
              ref={inputRef}
              className="composer-input"
              rows={1}
              value={draft}
              placeholder={
                canSend ? `Message #${channel.name}` : "You cannot post in this channel"
              }
              disabled={!canSend}
              onFocus={() => setFocused(true)}
              onBlur={() => setFocused(false)}
              onChange={(event) => {
                setDraft(event.target.value);
                void api.typing(channel.id).catch(() => {});
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  void send();
                }
                if (event.key === "Escape" && replyTo) setReplyTo(null);
              }}
            />
            {[...draft].length > MAX_CONTENT - 400 && (
              <span className="composer-left" data-over={left < 0}>
                {left} left
              </span>
            )}
            <button
              type="button"
              className="composer-icon"
              aria-label="Emoji"
              title="Emoji"
              onClick={(event) =>
                onOpenEmoji(null, event.currentTarget.getBoundingClientRect())
              }
            >
              <Icon name="smiley" size={21} weight="light" />
            </button>
            <button
              type="button"
              className="composer-icon"
              aria-label="Send"
              title="Send"
              disabled={!sendable}
              onClick={() => void send()}
            >
              <Icon name="paper-plane-tilt" size={18} />
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}

function MessageRow({
  message,
  grouped,
  highlighted,
  onMounted,
  onJumpTo,
  nameFor,
  meID,
  canReact,
  avatarURL,
  onReact,
  onUnreact,
  onOpenEmoji,
  onReply,
  onOpenMenu,
  onOpenImage,
}: {
  message: Message;
  grouped: boolean;
  highlighted: boolean;
  onMounted: (node: HTMLElement | null) => void;
  onJumpTo: (messageID: string) => void;
  nameFor: (id: string) => string;
  meID: string;
  canReact: boolean;
  avatarURL: (id: string) => string | null;
  onReact: (messageID: string, emoji: string) => void;
  onUnreact: (messageID: string, emoji: string) => void;
  onOpenEmoji: (messageID: string | null, anchor: DOMRect) => void;
  onReply: () => void;
  onOpenMenu: (message: Message, event: MouseEvent<HTMLElement>) => void;
  onOpenImage: (attachment: Attachment, message: Message) => void;
}) {
  const [hovered, setHovered] = useState(false);
  const name = nameFor(message.author.id);
  const reply = message.reply_to;
  const showHead = !grouped || Boolean(reply);

  return (
    <article
      ref={onMounted}
      className="message"
      data-grouped={grouped}
      data-reply={Boolean(reply)}
      data-highlighted={highlighted}
      onPointerEnter={() => setHovered(true)}
      onPointerLeave={() => setHovered(false)}
      onContextMenu={(event) => onOpenMenu(message, event)}
    >
      <div className="message-gutter">
        {showHead && (
          <Avatar
            name={name}
            url={avatarURL(message.author.id)}
            size={40}
            tone={message.author.id === meID ? "accent" : "neutral"}
          />
        )}
      </div>

      <div className="message-body">
        {reply && (
          <button
            type="button"
            className="reply-quote"
            disabled={reply.deleted}
            onClick={() => onJumpTo(reply.message_id)}
          >
            <span className="reply-quote-bar" />
            {reply.deleted ? (
              <span className="reply-quote-text">The message this answers is gone</span>
            ) : (
              <>
                <span className="reply-quote-author">
                  {reply.author ? reply.author.username : "Someone"}
                </span>
                <span className="reply-quote-text">
                  {reply.content}
                  {reply.truncated ? "…" : ""}
                </span>
              </>
            )}
          </button>
        )}

        {showHead && (
          <div className="message-head">
            <span className="message-author">{name}</span>
            <span className="message-time">{timeOf(message.created_at)}</span>
            {message.edited_at && <span className="message-edited">edited</span>}
          </div>
        )}

        {message.content && <div className="message-text">{message.content}</div>}

        {message.attachments.length > 0 && (
          <div className="attachments">
            {message.attachments.map((file) =>
              file.content_type.startsWith("image/") ? (
                <button
                  key={file.id}
                  type="button"
                  className="attachment-image"
                  onClick={() => onOpenImage(file, message)}
                >
                  <img src={file.url} alt={file.filename} />
                </button>
              ) : (
                <a
                  key={file.id}
                  className="attachment-file"
                  href={file.url}
                  download={file.filename}
                >
                  {file.filename}
                  <span className="attachment-size">{sizeOf(file.size_bytes)}</span>
                </a>
              ),
            )}
          </div>
        )}

        {message.reactions.length > 0 && (
          <div className="reactions">
            {message.reactions.map((reaction) => (
              <ReactionPill
                key={reaction.emoji}
                emoji={reaction.emoji}
                count={reaction.count}
                mine={reaction.mine}
                onClick={() =>
                  reaction.mine
                    ? onUnreact(message.id, reaction.emoji)
                    : onReact(message.id, reaction.emoji)
                }
              />
            ))}
            {canReact && (
              <button
                type="button"
                className="reaction-add"
                aria-label="Add a reaction"
                onClick={(event) =>
                  onOpenEmoji(message.id, event.currentTarget.getBoundingClientRect())
                }
              >
                <Icon name="smiley" size={12} weight="light" />
              </button>
            )}
          </div>
        )}
      </div>

      {hovered && (
        <div className="hover-toolbar">
          {canReact && (
            <IconButton
              name="smiley"
              state="plain"
              size={21}
              weight="light"
              label="React"
              onClick={(event) =>
                onOpenEmoji(message.id, event.currentTarget.getBoundingClientRect())
              }
            />
          )}
          <IconButton
            name="arrow-bend-up-left"
            state="plain"
            label="Reply"
            onClick={onReply}
          />
          <IconButton
            name="dots-three"
            state="plain"
            label="More"
            onClick={(event) => onOpenMenu(message, event)}
          />
        </div>
      )}
    </article>
  );
}

type Row =
  | { kind: "day"; key: string; label: string }
  | { kind: "message"; message: Message; grouped: boolean };

function groupMessages(messages: Message[]): Row[] {
  const rows: Row[] = [];
  let day = "";
  let lastAuthor = "";
  let lastAt = 0;

  for (const message of messages) {
    const at = new Date(message.created_at);
    const stamp = at.toDateString();
    if (stamp !== day) {
      day = stamp;
      lastAuthor = "";
      rows.push({ kind: "day", key: stamp, label: dayLabel(at) });
    }
    const close = at.getTime() - lastAt < 5 * 60 * 1000;
    const grouped =
      message.author.id === lastAuthor && close && !message.reply_to;
    rows.push({ kind: "message", message, grouped });
    lastAuthor = message.author.id;
    lastAt = at.getTime();
  }
  return rows;
}

function dayLabel(at: Date): string {
  const today = new Date();
  if (at.toDateString() === today.toDateString()) return "Today";
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (at.toDateString() === yesterday.toDateString()) return "Yesterday";
  return at.toLocaleDateString(undefined, { month: "long", day: "numeric" });
}

function timeOf(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function sizeOf(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function sentenceFor(names: string[]): string {
  if (names.length === 1) return `${names[0]} is typing`;
  if (names.length === 2) return `${names[0]} and ${names[1]} are typing`;
  return "Several people are typing";
}
