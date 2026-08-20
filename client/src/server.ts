const OVERRIDE_KEY = "server_url";
const DESKTOP_FALLBACK = "http://localhost:8080";

function servedOverHTTP(): boolean {
  return location.protocol === "http:" || location.protocol === "https:";
}

function override(): string | null {
  if (import.meta.env.VITE_API_URL) return import.meta.env.VITE_API_URL;
  return localStorage.getItem(OVERRIDE_KEY);
}

export function serverIsPinned(): boolean {
  return override() !== null;
}

export function serverIsEditable(): boolean {
  return !import.meta.env.VITE_API_URL;
}

export function defaultServerURL(): string {
  return servedOverHTTP() ? location.origin : DESKTOP_FALLBACK;
}

export function apiBase(): string {
  const configured = override();
  if (configured) return configured.replace(/\/+$/, "");
  if (servedOverHTTP()) return "";
  return DESKTOP_FALLBACK;
}

export function gatewayURL(): string {
  const base = apiBase();
  if (base) return base.replace(/^http/, "ws") + "/gateway";
  return `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/gateway`;
}

export function serverURL(): string {
  return apiBase() || location.origin;
}

export function setServerURL(url: string) {
  const trimmed = url.trim().replace(/\/+$/, "");
  if (trimmed) {
    localStorage.setItem(OVERRIDE_KEY, trimmed);
  } else {
    localStorage.removeItem(OVERRIDE_KEY);
  }
}
