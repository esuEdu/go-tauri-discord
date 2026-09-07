import { serverURL } from "./server";

const IN_PATH = /\/invite\/([^/?#\s]+)/;
const IN_QUERY = /[?&]invite=([^&#\s]+)/;

function codeIn(text: string): string | null {
  return IN_PATH.exec(text)?.[1] ?? IN_QUERY.exec(text)?.[1] ?? null;
}

const HELD = "pending_invite";

let pending = codeIn(location.pathname + location.search);

if (pending) {
  sessionStorage.setItem(HELD, pending);
  history.replaceState(null, "", "/");
} else {
  pending = sessionStorage.getItem(HELD);
}

export function inviteLink(code: string): string {
  return `${serverURL()}/invite/${code}`;
}

export function inviteCodeFrom(text: string): string {
  const trimmed = text.trim();
  return codeIn(trimmed) ?? trimmed;
}

export function takePendingInvite(): string | null {
  const held = pending;
  pending = null;
  sessionStorage.removeItem(HELD);
  return held;
}
