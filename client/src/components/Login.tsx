import { useEffect, useState } from "react";
import { api } from "../api";
import type { User } from "../types/events.gen";

interface Props {
  onAuthenticated: (u: User) => void;
  inviteCode?: string | null;
}

export function Login({ onAuthenticated, inviteCode }: Props) {
  const [invitedTo, setInvitedTo] = useState<string | null>(null);
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
      .catch(() => undefined);
    setMode("register");
    return () => {
      cancelled = true;
    };
  }, [inviteCode]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const user =
        mode === "login"
          ? await api.login(email, password)
          : await api.register(username, email, password);
      onAuthenticated(user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login">
      <form className="login-card" onSubmit={submit}>
        <h1>Vocalis</h1>
        <p className="muted">
          {invitedTo
            ? `You have been invited to ${invitedTo}.`
            : mode === "login"
              ? "Welcome back."
              : "Create an account."}
        </p>

        {mode === "register" && (
          <input
            placeholder="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoComplete="username"
            required
          />
        )}
        <input
          type="email"
          placeholder="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="email"
          required
        />
        <input
          type="password"
          placeholder="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete={mode === "login" ? "current-password" : "new-password"}
          required
        />

        {error && <div className="error">{error}</div>}

        <button type="submit" disabled={busy}>
          {busy ? "…" : mode === "login" ? "Log in" : "Register"}
        </button>
        <button
          type="button"
          className="link"
          onClick={() => {
            setMode(mode === "login" ? "register" : "login");
            setError(null);
          }}
        >
          {mode === "login" ? "Need an account?" : "Already have one?"}
        </button>
      </form>
    </div>
  );
}
