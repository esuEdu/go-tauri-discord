import { apiBase } from "../server";
import type { Attachment } from "../types/events.gen";

const IMAGES = ["image/png", "image/jpeg", "image/gif", "image/webp"];

function readableSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function Attachments({ attachments }: { attachments: Attachment[] }) {
  if (!attachments || attachments.length === 0) return null;

  return (
    <div className="attachments">
      {attachments.map((a) => {
        const href = apiBase() + a.url;
        if (IMAGES.includes(a.content_type)) {
          return (
            <a key={a.id} href={href} target="_blank" rel="noreferrer">
              <img className="attachment-image" src={href} alt={a.filename} loading="lazy" />
            </a>
          );
        }
        return (
          <a key={a.id} className="attachment-file" href={href} download={a.filename}>
            <span className="attachment-name">{a.filename}</span>
            <span className="muted"> {readableSize(a.size_bytes)}</span>
          </a>
        );
      })}
    </div>
  );
}
