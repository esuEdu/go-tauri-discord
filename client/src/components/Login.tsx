import { useEffect, useState } from "react";
import { api } from "../api";
import { Icon } from "./Icon";
import { ServerPicker } from "./ServerPicker";
import { initials } from "./Avatar";
import type { User } from "../types/events.gen";

const NAME_LIMIT = 32;

interface Props {
  onAuthenticated: (u: User) => void;
  inviteCode?: string | null;
}

export function Login({ onAuthenticated, inviteCode }: Props) {
  const [invitedTo, setInvitedTo] = useState<string | null>(null);
  const [inviteDead, setInviteDead] = useState(false);
  const [mode, setMode] = useState<"login" | "register">("login");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!inviteCode) return;
    let cancelled = false;
    api
      .previewInvite(inviteCode)
      .then((i) => !cancelled && setInvitedTo(i.guild_name))
      .catch(() => !cancelled && setInviteDead(true));
    setMode("register");
    return () => {
      cancelled = true;
    };
  }, [inviteCode]);

  const registering = mode === "register";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const user = registering
        ? await api.register(username, email, password)
        : await api.login(email, password);
      onAuthenticated(user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="gate">
      <div className="gate-stack">
        <form className="gate-card" onSubmit={submit}>
          <div className="gate-head">
            <span className="gate-badge">{initials(invitedTo ?? "Vocalis")}</span>
            <div className="grow">
              <div className="gate-title">
                {invitedTo ? invitedTo : registering ? "Make an account" : "Welcome back"}
              </div>
              <div className="gate-sub">{invitedTo ? "invited you" : "Vocalis"}</div>
            </div>
          </div>

          {registering && (
            <label className="field">
              Name
              <input
                className="input"
                placeholder="what people will call you"
                value={username}
                maxLength={NAME_LIMIT}
                autoComplete="username"
                required
                onChange={(e) => setUsername(e.target.value)}
              />
              <span className="field-note">
                {[...username].length} / {NAME_LIMIT} · spaces and emoji are fine. Four digits are
                added so a name can be shared.
              </span>
            </label>
          )}

          <label className="field">
            Email
            <input
              className="input"
              type="email"
              placeholder="you@example.com"
              value={email}
              autoComplete="email"
              required
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>

          <label className="field">
            Password
            <input
              className="input"
              type="password"
              placeholder={registering ? "at least 8 characters" : "••••••••"}
              value={password}
              autoComplete={registering ? "new-password" : "current-password"}
              required
              onChange={(e) => setPassword(e.target.value)}
            />
            {registering && <span className="field-note">Length is the only rule.</span>}
          </label>

          {error && (
            <div className="banner">
              <Icon name="warning-circle" size={16} />
              <span>{error}</span>
            </div>
          )}

          <button className="btn btn-primary btn-block" type="submit" disabled={busy}>
            {busy ? "…" : registering ? "Create account" : "Sign in"}
          </button>

          <div className="gate-foot">
            <span>
              {registering ? "Already have one? " : "New here? "}
              <button
                type="button"
                className="link"
                onClick={() => {
                  setMode(registering ? "login" : "register");
                  setError(null);
                }}
              >
                {registering ? "Sign in" : "Make an account"}
              </button>
            </span>
            <span className="muted">Lost your password?</span>
          </div>
        </form>

        {invitedTo && (
          <div className="gate-aside">
            <div className="note bright">
              Make an account and you are in. Already have one? Sign in — you still land in the
              server.
            </div>
          </div>
        )}

        {inviteDead && (
          <div className="gate-aside">
            <div className="row">
              <Icon name="warning-circle" size={16} />
              <span className="gate-aside-title">This invite no longer works</span>
            </div>
            <div className="note">
              It expired, ran out of uses, or was taken back. Ask whoever sent it for a fresh link —
              you can still make an account here.
            </div>
          </div>
        )}

        <ServerPicker />
      </div>
    </div>
  );
}
