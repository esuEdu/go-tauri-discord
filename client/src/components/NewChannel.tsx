import { useEffect, useState } from "react";
import { api } from "../api";
import { Icon } from "./Icon";
import type { Channel, Guild } from "../types/events.gen";

type Kind = "text" | "voice" | "category";

const KINDS: { id: Kind; name: string; about: string; icon: string }[] = [
  { id: "text", name: "Text", about: "messages and files", icon: "hash" },
  { id: "voice", name: "Voice", about: "talking and screens", icon: "speaker-high" },
  { id: "category", name: "Category", about: "a heading the others fold under", icon: "folder-simple" },
];

export function NewChannel({
  guild,
  parent,
  taken,
  onClose,
}: {
  guild: Guild;
  parent: Channel | null;
  taken: number;
  onClose: () => void;
}) {
  const [kind, setKind] = useState<Kind>("text");
  const [name, setName] = useState("");
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
    if (!name.trim()) {
      setError("It needs a name.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await api.createChannel(
        guild.id,
        name.trim(),
        kind,
        taken,
        kind === "category" ? undefined : (parent?.id ?? undefined),
      );
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not create the channel");
      setBusy(false);
    }
  }

  return (
    <div className="dialog-backdrop" onMouseDown={() => !busy && onClose()}>
      <form
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label={`New in ${guild.name}`}
        onMouseDown={(e) => e.stopPropagation()}
        onSubmit={submit}
      >
        <div>
          <div className="dialog-title">New in {guild.name}</div>
          <div className="dialog-sub">
            {parent ? `under ${parent.name}` : "above the first heading"}
          </div>
        </div>

        <div className="stack tight">
          {KINDS.map((k) => (
            <button
              key={k.id}
              type="button"
              className={k.id === kind ? "choice picked" : "choice"}
              onClick={() => setKind(k.id)}
            >
              <Icon name={k.icon} size={16} />
              <span className="grow">
                <span className="choice-name">{k.name}</span>
                <span className="choice-about">{k.about}</span>
              </span>
              {k.id === kind && <Icon name="check" size={14} />}
            </button>
          ))}
        </div>

        <label className="field">
          Name
          <input
            className="input"
            autoFocus
            placeholder={kind === "category" ? "Voice" : "build-log"}
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setError(null);
            }}
          />
          <span className="field-note">
            The only rule the server has: a channel must be called something. Anything else is
            allowed, spaces and all.
          </span>
        </label>

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
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? "…" : "Create"}
          </button>
        </div>
      </form>
    </div>
  );
}
