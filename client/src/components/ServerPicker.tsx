import { useState } from "react";
import { api } from "../api";
import {
  defaultServerURL,
  serverIsEditable,
  serverIsPinned,
  serverURL,
  setServerURL,
} from "../server";

export function ServerPicker() {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(serverURL());
  const [error, setError] = useState<string | null>(null);

  const current = serverURL();
  const pinned = serverIsPinned();

  function apply(url: string) {
    const trimmed = url.trim();
    if (trimmed) {
      try {
        new URL(trimmed);
      } catch {
        setError("that is not a valid URL");
        return;
      }
    }
    if ((trimmed || defaultServerURL()) !== current) api.clear();
    setServerURL(trimmed);
    location.reload();
  }

  if (!serverIsEditable()) {
    return (
      <p className="muted server-line">
        Server: <code>{current}</code> (fixed at build time)
      </p>
    );
  }

  if (!open) {
    return (
      <p className="muted server-line">
        Server: <code>{current}</code>{" "}
        <button
          type="button"
          className="link"
          onClick={() => {
            setDraft(current);
            setError(null);
            setOpen(true);
          }}
        >
          Change
        </button>
        {pinned && (
          <>
            {" "}
            <button type="button" className="link" onClick={() => apply("")}>
              Reset
            </button>
          </>
        )}
      </p>
    );
  }

  return (
    <div className="server-edit">
      <label htmlFor="server-url">Server address</label>
      <input
        id="server-url"
        type="url"
        inputMode="url"
        placeholder={defaultServerURL()}
        value={draft}
        autoFocus
        onChange={(e) => {
          setDraft(e.target.value);
          setError(null);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            apply(draft);
          }
          if (e.key === "Escape") setOpen(false);
        }}
      />
      {error && <div className="error inline">{error}</div>}
      <div className="server-actions">
        <button type="button" className="secondary" onClick={() => setOpen(false)}>
          Cancel
        </button>
        <button type="button" className="link" onClick={() => apply("")}>
          Use default
        </button>
        <button type="button" onClick={() => apply(draft)}>
          Connect
        </button>
      </div>
    </div>
  );
}
