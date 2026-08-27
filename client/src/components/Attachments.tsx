import { useState } from "react";
import { apiBase } from "../server";
import { Icon } from "./Icon";
import type { Attachment } from "../types/events.gen";

const IMAGES = ["image/png", "image/jpeg", "image/gif", "image/webp"];

function readableSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function Picture({ href, name }: { href: string; name: string }) {
  const [broken, setBroken] = useState(false);

  if (broken) {
    return (
      <div className="attachment-gone">
        <Icon name="image-broken" size={17} />
        <span className="note">
          A link to a file stops working a day after the app asked for it. Reload the window to see{" "}
          {name} again.
        </span>
      </div>
    );
  }

  return (
    <a href={href} target="_blank" rel="noreferrer">
      <img
        className="attachment-image"
        src={href}
        alt={name}
        loading="lazy"
        onError={() => setBroken(true)}
      />
    </a>
  );
}

export function Attachments({ attachments }: { attachments: Attachment[] }) {
  if (!attachments || attachments.length === 0) return null;

  return (
    <div className="attachments">
      {attachments.map((a) => {
        const href = apiBase() + a.url;
        if (IMAGES.includes(a.content_type)) {
          return <Picture key={a.id} href={href} name={a.filename} />;
        }
        return (
          <a key={a.id} className="attachment-file" href={href} download={a.filename}>
            <Icon name="file" size={16} />
            <span>{a.filename}</span>
            <span className="attachment-size">{readableSize(a.size_bytes)}</span>
          </a>
        );
      })}
    </div>
  );
}
