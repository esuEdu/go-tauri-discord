import { useState, type FormEvent } from "react";
import { api, ApiError } from "../api";
import type { User } from "../types/events.gen";
import { Avatar } from "../ui/Avatar";
import { Button } from "../ui/Button";

type Mode = "login" | "register" | "forgot";

const HEADINGS: Record<Mode, string> = {
  login: "Welcome back",
  register: "Make an account",
  forgot: "Lost your password?",
};

export function Auth({ onSignedIn }: { onSignedIn: (user: User) => void }) {
  const [mode, setMode] = useState<Mode>("login");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function go(next: Mode) {
    setMode(next);
    setError(null);
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy || mode === "forgot") return;
    setBusy(true);
    setError(null);
    try {
      const user =
        mode === "login"
          ? await api.login(email, password)
          : await api.register(username, email, password);
      onSignedIn(user);
    } catch (cause) {
      setError(messageFor(mode, cause));
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <form className="auth-card" onSubmit={submit}>
        <div className="auth-head">
          <Avatar name="Vocalis" size={34} tone="accent" />
          <div className="auth-head-text">
            <span className="auth-title">{HEADINGS[mode]}</span>
            <span className="auth-brand">Vocalis</span>
          </div>
        </div>

        {mode === "register" && (
          <label className="field">
            <span className="field-label">Name</span>
            <input
              className="input"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
            />
            <span className="field-hint">
              Spaces and emoji are fine. It cannot be changed later.
            </span>
          </label>
        )}

        <label className="field">
          <span className="field-label">Email</span>
          <input
            className="input"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            autoFocus={mode !== "register"}
          />
        </label>

        {mode !== "forgot" && (
          <label className="field">
            <span className="field-label">Password</span>
            <input
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={mode === "login" ? "current-password" : "new-password"}
            />
            {mode === "register" && (
              <span className="field-hint">Length is the only rule.</span>
            )}
          </label>
        )}

        {error && <span className="error-text">{error}</span>}

        {mode === "forgot" ? (
          <div className="needs-backend">
            Sending a reset link needs a mail server, and this one has none yet.
            Nothing is sent.
          </div>
        ) : (
          <Button type="submit" disabled={busy} className="auth-submit">
            {mode === "login" ? "Sign in" : "Create account"}
          </Button>
        )}

        <div className="auth-links">
          {mode === "login" ? (
            <>
              <button type="button" className="auth-link" onClick={() => go("register")}>
                New here? Make an account
              </button>
              <button
                type="button"
                className="auth-link auth-link-quiet"
                onClick={() => go("forgot")}
              >
                Lost your password?
              </button>
            </>
          ) : (
            <button type="button" className="auth-link" onClick={() => go("login")}>
              Already have one? Sign in
            </button>
          )}
        </div>
      </form>
    </div>
  );
}

function messageFor(mode: Mode, cause: unknown): string {
  const status = cause instanceof ApiError ? cause.status : 0;
  if (mode === "login") {
    if (status === 401 || status === 404) return "That email and password do not match.";
    if (status === 429) return "Too many attempts. Wait a moment and try again.";
    return "Could not sign in. The server did not answer.";
  }
  if (status === 409) return "That email is already in use.";
  if (status === 400) return "Check the name, email and password.";
  if (status === 429) return "Too many attempts. Wait a moment and try again.";
  return "Could not make the account. The server did not answer.";
}
