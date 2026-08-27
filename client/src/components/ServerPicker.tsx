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
      <div className="gate-foot">
        <span className="muted">
          Server <span className="mono">{current}</span>
        </span>
        <span className="muted">fixed at build time</span>
      </div>
    );
  }

  if (!open) {
    return (
      <div className="gate-foot">
        <span className="muted clip">
          Server <span className="mono">{current}</span>
        </span>
        <span className="row">
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
          {serverIsPinned() && (
            <button type="button" className="link quiet" onClick={() => apply("")}>
              Reset
            </button>
          )}
        </span>
      </div>
    );
  }

  return (
    <div className="gate-card">
      <label className="field">
        Server address
        <input
          className="input"
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
      </label>

      {error && <span className="field-note">{error}</span>}

      <div className="dialog-actions">
        <button type="button" className="btn btn-quiet" onClick={() => setOpen(false)}>
          Cancel
        </button>
        <button type="button" className="btn btn-quiet" onClick={() => apply("")}>
          Use default
        </button>
        <button type="button" className="btn btn-primary" onClick={() => apply(draft)}>
          Connect
        </button>
      </div>
    </div>
  );
}
