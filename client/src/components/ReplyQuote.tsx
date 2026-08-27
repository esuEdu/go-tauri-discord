import { session } from "../session";
import { Icon } from "./Icon";
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
      <div className="reply-line gone">
        <span className="reply-mark" aria-hidden="true" />
        Original message was deleted
      </div>
    );
  }

  const body = (
    <>
      <span className="reply-mark" aria-hidden="true" />
      {reply.author && (
        <span className="reply-line-author">{session.labelOf(reply.author.id)}</span>
      )}
      <span className="reply-line-text">
        {reply.content}
        {reply.truncated && "…"}
      </span>
      {reply.has_attachments && <Icon name="paperclip" size={12} />}
    </>
  );

  if (!onJump) return <div className="reply-line">{body}</div>;

  return (
    <button className="reply-line" onClick={() => onJump(reply.message_id)}>
      {body}
    </button>
  );
}
