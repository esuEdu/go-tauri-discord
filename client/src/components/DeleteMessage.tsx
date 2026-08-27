import { useEffect, useState } from "react";
import { api } from "../api";
import { Icon } from "./Icon";
import type { Message } from "../types/events.gen";

export function DeleteMessage({ message, onClose }: { message: Message; onClose: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  async function remove() {
    setBusy(true);
    setError(null);
    try {
      await api.deleteMessage(message.id);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not delete it");
      setBusy(false);
    }
  }

  const files = message.attachments?.length ?? 0;

  return (
    <div className="dialog-backdrop" onMouseDown={() => !busy && onClose()}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Delete this message?"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="dialog-title">Delete this message?</div>

        {message.content && <div className="dialog-quote">“{message.content}”</div>}

        <div className="dialog-text">
          It goes for everyone
          {files > 0 && `, along with its ${files === 1 ? "file" : `${files} files`}`} and its
          reactions. Anything that replied to it will read{" "}
          <strong>Original message was deleted</strong>. This cannot be undone.
        </div>

        {error && (
          <div className="banner bad">
            <Icon name="warning-circle" size={15} />
            <span>{error}</span>
          </div>
        )}

        <div className="dialog-actions">
          <button className="btn btn-quiet" onClick={onClose} disabled={busy}>
            Keep it
          </button>
          <button className="btn btn-danger" onClick={() => void remove()} disabled={busy}>
            {busy ? "…" : "Delete"}
          </button>
        </div>
      </div>
    </div>
  );
}
