import { useEffect, useState } from "react";
import type { Attachment } from "../types/events.gen";
import { mediaURL } from "../server";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { Menu, MenuItem } from "../ui/Menu";

function sizeOf(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function Lightbox({
  attachment,
  uploader,
  uploaderAvatarURL,
  postedAt,
  onClose,
}: {
  attachment: Attachment;
  uploader: string;
  uploaderAvatarURL: string | null;
  postedAt: string;
  onClose: () => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  async function copyImage() {
    setMenuOpen(false);
    const blob = await fetch(mediaURL(attachment.url)).then((r) => r.blob());
    await navigator.clipboard.write([new ClipboardItem({ [blob.type]: blob })]);
  }

  async function copyLink() {
    setMenuOpen(false);
    await navigator.clipboard.writeText(mediaURL(attachment.url));
  }

  return (
    <div className="lightbox" onClick={onClose}>
      <div className="lightbox-bar" onClick={(event) => event.stopPropagation()}>
        <div className="lightbox-who">
          <Avatar name={uploader} url={uploaderAvatarURL} size={28} className="lightbox-avatar" />
          <div className="lightbox-who-text">
            <span className="lightbox-filename">{attachment.filename}</span>
            <span className="lightbox-when">
              {uploader} · {postedAt} · {sizeOf(attachment.size_bytes)}
            </span>
          </div>
        </div>
        <div className="lightbox-actions">
          <a
            className="lightbox-action"
            href={mediaURL(attachment.url)}
            download={attachment.filename}
            aria-label="Download"
            title="Download"
          >
            <Icon name="download" size={18} />
          </a>
          <span className="lightbox-divider" />
          <div className="lightbox-action-wrap">
            <button
              type="button"
              className="lightbox-action"
              aria-label="More"
              title="More"
              onClick={() => setMenuOpen((was) => !was)}
            >
              <Icon name="dots-three" size={18} />
            </button>
            {menuOpen && (
              <div className="lightbox-menu">
                <Menu>
                  <MenuItem icon="copy" label="Copy Image" onClick={() => void copyImage()} />
                  <MenuItem icon="copy" label="Copy Link" onClick={() => void copyLink()} />
                  <MenuItem
                    icon="download"
                    label="Save Image As"
                    onClick={() => {
                      setMenuOpen(false);
                      const link = document.createElement("a");
                      link.href = mediaURL(attachment.url);
                      link.download = attachment.filename;
                      link.click();
                    }}
                  />
                </Menu>
              </div>
            )}
          </div>
          <span className="lightbox-divider" />
          <button
            type="button"
            className="lightbox-action"
            aria-label="Close"
            title="Close"
            onClick={onClose}
          >
            <Icon name="x" size={18} />
          </button>
        </div>
      </div>

      <img
        className="lightbox-picture"
        src={mediaURL(attachment.url)}
        alt={attachment.filename}
        onClick={(event) => event.stopPropagation()}
      />
    </div>
  );
}
