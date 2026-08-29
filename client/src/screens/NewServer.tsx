import { useState } from "react";
import { api, ApiError } from "../api";
import type { Guild } from "../types/events.gen";
import { Avatar } from "../ui/Avatar";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Sheet } from "../ui/Sheet";

type Step = "pick" | "name" | "code";

export function NewServer({
  onClose,
  onArrived,
}: {
  onClose: () => void;
  onArrived: (guild: Guild) => void;
}) {
  const [step, setStep] = useState<Step>("pick");
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function run(work: () => Promise<Guild>) {
    setBusy(true);
    setError(null);
    try {
      onArrived(await work());
    } catch (cause) {
      setError(reasonFor(cause));
      setBusy(false);
    }
  }

  if (step === "pick") {
    return (
      <Sheet
        title="A place for the group"
        subtitle="Start one and invite people, or step into one you were given a code for."
        onClose={onClose}
      >
        <button
          type="button"
          className="choice"
          data-selected="true"
          onClick={() => setStep("name")}
        >
          <span className="choice-icon">
            <Icon name="plus" size={16} />
          </span>
          <span className="choice-text">
            <span className="choice-name">Make my own</span>
            <span className="choice-desc">You will own it</span>
          </span>
        </button>
        <button type="button" className="choice" onClick={() => setStep("code")}>
          <span className="choice-icon">
            <Icon name="monitor-arrow-up" size={16} />
          </span>
          <span className="choice-text">
            <span className="choice-name">Join with a code</span>
            <span className="choice-desc">Paste it, or open the link</span>
          </span>
        </button>
      </Sheet>
    );
  }

  if (step === "name") {
    return (
      <Sheet title="Name it" onClose={onClose}>
        <div className="born-row">
          <Avatar name={name.trim() || "?"} size={56} tone="accent" />
          <p className="born-text">
            Its icon, for now. Every server is born with the first two letters of its name —
            a picture can be added afterwards, in its settings.
          </p>
        </div>

        <label className="field">
          <span className="field-label">Name</span>
          <span className="counted">
            <input
              className="input"
              value={name}
              placeholder="Sunday Roast"
              maxLength={100}
              autoFocus
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && name.trim()) {
                  void run(() => api.createGuild(name.trim()));
                }
              }}
            />
            <span className="counted-tally">{name.length} / 100</span>
          </span>
        </label>
        {error && <span className="error-text">{error}</span>}
        <div className="sheet-actions sheet-actions-left">
          <Button kind="quiet" onClick={() => setStep("pick")}>
            Back
          </Button>
          <Button
            disabled={busy || !name.trim()}
            onClick={() => void run(() => api.createGuild(name.trim()))}
          >
            Make it
          </Button>
        </div>
      </Sheet>
    );
  }

  return (
    <Sheet title="Join with a code" className="sheet-join" onClose={onClose}>
      <label className="field">
        <span className="field-label">Code or link</span>
        <input
          className="input"
          value={code}
          placeholder="7Kq2xR"
          autoFocus
          onChange={(event) => setCode(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && code.trim()) {
              void run(() => api.redeemInvite(code.trim()));
            }
          }}
        />
      </label>
      <Button
        className="wide-button"
        disabled={busy || !code.trim()}
        onClick={() => void run(() => api.redeemInvite(code.trim()))}
      >
        Step in
      </Button>
      {error && <p className="sheet-error">{error}</p>}
      <p className="sheet-note">
        If you are already in that server, joining again simply opens it.
      </p>
    </Sheet>
  );
}

function reasonFor(cause: unknown): string {
  const status = cause instanceof ApiError ? cause.status : 0;
  if (status === 404) return "No invite with that code.";
  if (status === 410) return "That invite has expired or run out of uses.";
  if (status === 403) return "You are banned from that server.";
  if (status === 409) return "You are already in that server.";
  if (status === 400) return "Check the code and try again.";
  return "The server did not answer.";
}
