import { useEffect, useState } from "react";
import { api } from "../api";
import { Icon } from "./Icon";

export function DeleteAccount({
  onDeleted,
  onClose,
}: {
  onDeleted: () => void;
  onClose: () => void;
}) {
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

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

  return (
    <div className="dialog-backdrop" onMouseDown={() => !busy && onClose()}>
      <form
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Delete your account"
        onMouseDown={(e) => e.stopPropagation()}
        onSubmit={submit}
      >
        <div className="dialog-title">Delete your account</div>

        <div className="stack tight">
          <span className="dialog-text">
            What you wrote stays, under <strong>Deleted User</strong>.
          </span>
          <span className="dialog-text">Servers you own pass to another member.</span>
          <span className="dialog-text">Every device you are signed in on is signed out.</span>
          <span className="dialog-text">
            <strong>It cannot be undone.</strong>
          </span>
        </div>

        <label className="field">
          Your password
          <input
            className="input"
            type="password"
            autoFocus
            autoComplete="current-password"
            placeholder="••••••••"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>

        {error && (
          <div className="banner bad">
            <Icon name="warning-circle" size={15} />
            <span>{error}</span>
          </div>
        )}

        <div className="dialog-actions">
          <button type="button" className="btn btn-quiet" onClick={onClose} disabled={busy}>
            Keep it
          </button>
          <button type="submit" className="btn btn-danger" disabled={busy || password === ""}>
            {busy ? "…" : "Delete for good"}
          </button>
        </div>
      </form>
    </div>
  );
}
