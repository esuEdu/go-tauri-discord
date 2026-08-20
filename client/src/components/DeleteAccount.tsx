import { useEffect, useState } from "react";
import { api } from "../api";

export function DeleteAccount({ onDeleted }: { onDeleted: () => void }) {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  function close() {
    setOpen(false);
    setPassword("");
    setError(null);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.deleteAccount(password);
      onDeleted();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not delete the account");
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button className="link danger" onClick={() => setOpen(true)}>
        Delete account
      </button>
    );
  }

  return (
    <div className="dialog-backdrop" onMouseDown={close}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="delete-account-title"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <h2 id="delete-account-title">Delete your account?</h2>

        <p className="muted">This cannot be undone. When it is done:</p>
        <ul className="muted">
          <li>Your messages stay where they are, shown as sent by “Deleted User”.</li>
          <li>Servers you own pass to another member, or are deleted if you are the only one left.</li>
          <li>You are signed out everywhere, and your sign-in stops working.</li>
        </ul>

        <form onSubmit={submit}>
          <label htmlFor="delete-account-password">Confirm your password</label>
          <input
            id="delete-account-password"
            type="password"
            autoFocus
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />

          {error && <div className="error inline">{error}</div>}

          <div className="dialog-actions">
            <button type="button" className="secondary" onClick={close} disabled={busy}>
              Keep my account
            </button>
            <button type="submit" className="danger" disabled={busy || password === ""}>
              {busy ? "Deleting…" : "Delete for good"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
