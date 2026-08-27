import { useEffect, useState } from "react";
import { api } from "../api";
import { session } from "../session";
import { Avatar } from "./Avatar";
import { Icon } from "./Icon";
import type { Guild } from "../types/events.gen";

export function RemoveMember({
  guild,
  userID,
  name,
  ban,
  onClose,
}: {
  guild: Guild;
  userID: string;
  name: string;
  ban: boolean;
  onClose: () => void;
}) {
  const [reason, setReason] = useState("");
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
      if (ban) await api.ban(guild.id, userID, reason);
      else await api.kick(guild.id, userID);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not remove them");
      setBusy(false);
    }
  }

  return (
    <div className="dialog-backdrop" onMouseDown={() => !busy && onClose()}>
      <form
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label={ban ? `Ban ${name}?` : `Remove ${name}?`}
        onMouseDown={(e) => e.stopPropagation()}
        onSubmit={submit}
      >
        <div className="row">
          <Avatar name={session.nameOf(userID)} imageKey={null} className="big" />
          <div className="grow">
            <div className="dialog-title">
              {ban ? `Ban ${name}?` : `Remove ${name} from ${guild.name}?`}
            </div>
            <div className="dialog-sub">
              They lose the server as they are looking at it, and drop out of any call.
            </div>
          </div>
        </div>

        {ban && (
          <label className="field">
            Reason — optional, kept for whoever looks later
            <textarea
              className="input"
              autoFocus
              maxLength={500}
              placeholder="up to 500 characters"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </label>
        )}

        <div className="panel">
          <span className="dialog-text">What they wrote stays where it is, still shown as theirs.</span>
          <span className="dialog-text">
            {ban
              ? "A ban is by account, and nothing here checks an email address. They can register again in a minute and use a new invite."
              : "Nothing stops them coming back with a fresh invite."}
          </span>
        </div>

        {error && (
          <div className="banner bad">
            <Icon name="warning-circle" size={15} />
            <span>{error}</span>
          </div>
        )}

        <div className="dialog-actions">
          <button type="button" className="btn btn-quiet" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn btn-danger" disabled={busy}>
            {busy ? "…" : ban ? "Ban them" : "Remove them"}
          </button>
        </div>
      </form>
    </div>
  );
}
