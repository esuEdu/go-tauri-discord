import { Avatar } from "./Avatar";
import { session } from "../session";
import type { ReplyPreview } from "../types/events.gen";

export function ReplyQuote({
  reply,
  onJump,
}: {
  reply: ReplyPreview;
  onJump?: (messageID: string) => void;
}) {
  if (reply.deleted) {
    return (
      <div className="reply-quote muted">
        <span className="reply-mark" aria-hidden="true" />
        Original message was deleted
      </div>
    );
  }

  const body = (
    <>
      <span className="reply-mark" aria-hidden="true" />
      {reply.author && (
        <>
          <Avatar
            name={reply.author.username}
            imageKey={reply.author.avatar_key}
            className="avatar reply"
          />
          <span className="reply-author">{session.labelOf(reply.author.id)}</span>
        </>
      )}
      <span className="reply-content muted">
        {reply.content}
        {reply.truncated && "…"}
        {reply.has_attachments && <span className="reply-clip"> 📎</span>}
      </span>
    </>
  );

  if (!onJump) return <div className="reply-quote">{body}</div>;

  return (
    <button className="reply-quote link" onClick={() => onJump(reply.message_id)}>
      {body}
    </button>
  );
}
