import { useEffect, useState } from "react";
import { api } from "../api";
import type { Guild } from "../types/events.gen";

export function RemoveMember({ guild, userID, name, ban, onClose }: {
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
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="remove-member-title"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <h2 id="remove-member-title">
          {ban ? `Ban ${name} from ${guild.name}?` : `Remove ${name} from ${guild.name}?`}
        </h2>

        <ul className="muted">
          <li>They lose the server straight away, and leave any voice channel they are in.</li>
          <li>What they wrote stays where it is, still shown as theirs.</li>
          {ban ? (
            <li>No invite will let them back in until you lift the ban.</li>
          ) : (
            <li>Nothing stops them coming back with a new invite.</li>
          )}
        </ul>

        {ban && (
          <p className="muted">
            A ban is by account. Nothing here verifies an email address, so somebody determined
            can register again and use a fresh invite.
          </p>
        )}

        <form onSubmit={submit}>
          {ban && (
            <>
              <label htmlFor="remove-member-reason">Reason (optional)</label>
              <input
                id="remove-member-reason"
                autoFocus
                maxLength={500}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </>
          )}

          {error && <div className="error inline">{error}</div>}

          <div className="dialog-actions">
            <button type="button" className="secondary" onClick={onClose} disabled={busy}>
              Cancel
            </button>
            <button type="submit" className="danger" disabled={busy}>
              {busy ? "Working…" : ban ? "Ban them" : "Remove them"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
