import { useEffect, useState } from "react";
import { api } from "../api";
import { Icon } from "./Icon";
import { initials } from "./Avatar";

const NAME_LIMIT = 100;

type Step = "pick" | "make" | "join";

export function NewServer({ onClose, onMade }: { onClose: () => void; onMade: () => void }) {
  const [step, setStep] = useState<Step>("pick");
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onClose]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.createGuild(name.trim());
      onMade();
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not create it");
      setBusy(false);
    }
  }

  async function join(e: React.FormEvent) {
    e.preventDefault();
    if (!code.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.redeemInvite(code.trim());
      onMade();
    } catch (err) {
      setError(err instanceof Error ? err.message : "that code does not work any more");
      setBusy(false);
    }
  }

  return (
    <div className="dialog-backdrop" onMouseDown={() => !busy && onClose()}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label="A place for the group"
        onMouseDown={(e) => e.stopPropagation()}
      >
        {step === "pick" && (
          <>
            <div>
              <div className="dialog-title">A place for the group</div>
              <div className="dialog-sub">
                Start one and invite people, or step into one you were given a code for.
              </div>
            </div>

            <button className="choice picked" onClick={() => setStep("make")}>
              <Icon name="plus" size={18} />
              <span className="grow">
                <span className="choice-name">Make my own</span>
                <span className="choice-about">You will own it</span>
              </span>
              <Icon name="arrow-right" size={15} />
            </button>

            <button className="choice" onClick={() => setStep("join")}>
              <Icon name="link" size={18} />
              <span className="grow">
                <span className="choice-name">Join with a code</span>
                <span className="choice-about">Paste it, or open the link</span>
              </span>
              <Icon name="arrow-right" size={15} />
            </button>
          </>
        )}

        {step === "make" && (
          <form className="stack" onSubmit={create}>
            <div className="dialog-title">Name it</div>

            <div className="panel">
              <div className="row">
                <span className="avatar big mine">{initials(name || "?")}</span>
                <span className="field-note grow">
                  Its icon, for now. Every server is born with the first two letters of its name — a
                  picture can be added afterwards, in its settings.
                </span>
              </div>
            </div>

            <label className="field">
              Name
              <input
                className="input"
                autoFocus
                maxLength={NAME_LIMIT}
                placeholder="Sunday Roast"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <span className="field-note">
                {[...name].length} / {NAME_LIMIT} · no description, no template, no picture at this
                point — those are all the server knows about a new one.
              </span>
            </label>

            {error && (
              <div className="banner bad">
                <Icon name="warning-circle" size={15} />
                <span>{error}</span>
              </div>
            )}

            <div className="dialog-actions">
              <button type="button" className="btn btn-quiet" onClick={() => setStep("pick")}>
                Back
              </button>
              <button type="submit" className="btn btn-primary" disabled={busy || !name.trim()}>
                {busy ? "…" : "Create it"}
              </button>
            </div>
          </form>
        )}

        {step === "join" && (
          <form className="stack" onSubmit={join}>
            <div className="dialog-title">Join with a code</div>

            <label className="field">
              Code or link
              <input
                className="input mono"
                autoFocus
                placeholder="7Kq2xR"
                value={code}
                onChange={(e) => setCode(e.target.value)}
              />
            </label>

            {error && (
              <div className="banner">
                <Icon name="warning-circle" size={15} />
                <span>{error} It expired, ran out of uses, or was taken back.</span>
              </div>
            )}

            <span className="field-note">
              If you are already in that server, joining again simply opens it.
            </span>

            <div className="dialog-actions">
              <button type="button" className="btn btn-quiet" onClick={() => setStep("pick")}>
                Back
              </button>
              <button type="submit" className="btn btn-primary" disabled={busy || !code.trim()}>
                {busy ? "…" : "Join"}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
