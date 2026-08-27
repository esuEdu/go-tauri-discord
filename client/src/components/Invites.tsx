import { useCallback, useEffect, useState } from "react";
import { api, type Invite } from "../api";
import { Icon } from "./Icon";
import type { Guild } from "../types/events.gen";

function linkFor(code: string): string {
  return `${location.origin}/?invite=${code}`;
}

export function Invites({ guild, onClose }: { guild: Guild; onClose: () => void }) {
  const [invites, setInvites] = useState<Invite[]>([]);
  const [maxUses, setMaxUses] = useState("");
  const [expiresIn, setExpiresIn] = useState("");
  const [made, setMade] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setInvites(await api.invites(guild.id));
    } catch {
      setInvites([]);
    }
  }, [guild.id]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  async function make() {
    setBusy(true);
    setError(null);
    try {
      const uses = Number(maxUses);
      const hours = Number(expiresIn);
      const invite = await api.createInvite(
        guild.id,
        uses > 0 ? uses : undefined,
        hours > 0 ? hours : undefined,
      );
      setMade(invite.code);
      await copy(invite.code);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not make a link");
    } finally {
      setBusy(false);
    }
  }

  async function copy(code: string) {
    try {
      await navigator.clipboard.writeText(linkFor(code));
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
    }
  }

  async function revoke(code: string) {
    setRevoking(code);
    setError(null);
    try {
      await api.revokeInvite(code);
      if (made === code) setMade(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not revoke it");
    } finally {
      setRevoking(null);
    }
  }

  return (
    <div className="dialog-backdrop" onMouseDown={onClose}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label={`Invite people to ${guild.name}`}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="dialog-title">Invite somebody to {guild.name}</div>

        <div className="dashed">
          <div className="note">
            Inviting by name and digits — <span className="mono">Rui#0317</span> — is drawn but not
            built; the server has no way to look somebody up by hand yet.
          </div>
          <span className="pending">invite by name · needs the server</span>
        </div>

        <div className="kicker">Or a link anyone can use</div>

        <div className="row">
          <label className="field grow">
            Uses
            <input
              className="input"
              type="number"
              min={1}
              placeholder="any number"
              value={maxUses}
              onChange={(e) => setMaxUses(e.target.value)}
            />
          </label>
          <label className="field grow">
            Expires after
            <input
              className="input"
              type="number"
              min={1}
              placeholder="never"
              value={expiresIn}
              onChange={(e) => setExpiresIn(e.target.value)}
            />
          </label>
        </div>

        <button className="btn btn-primary btn-block" disabled={busy} onClick={() => void make()}>
          {busy ? "…" : "Make a link"}
        </button>

        {made && (
          <button className="invite-link" onClick={() => void copy(made)}>
            <span className="mono grow clip">{linkFor(made)}</span>
            <Icon name={copied ? "check" : "copy"} size={16} />
          </button>
        )}

        {error && (
          <div className="banner bad">
            <Icon name="warning-circle" size={15} />
            <span>{error}</span>
          </div>
        )}

        <div className="stack tight">
          <div className="kicker">Live invites</div>
          {invites.length === 0 && (
            <span className="field-note">None yet. A link lasts until you revoke it.</span>
          )}
          {invites.map((invite) => (
            <div
              key={invite.code}
              className={revoking === invite.code ? "list-row fading" : "list-row"}
            >
              <span className="mono grow clip">{invite.code}</span>
              <span className="muted">
                {invite.uses}
                {invite.max_uses ? ` of ${invite.max_uses}` : ""}
              </span>
              <button
                className="link quiet"
                disabled={revoking === invite.code}
                onClick={() => void revoke(invite.code)}
              >
                {revoking === invite.code ? "revoking…" : "Revoke"}
              </button>
            </div>
          ))}
          <span className="field-note">
            Revoking one does not remove anybody who already joined.
          </span>
        </div>

        <div className="dialog-actions">
          <button className="btn btn-quiet" onClick={onClose}>
            Done
          </button>
        </div>
      </div>
    </div>
  );
}
