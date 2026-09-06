export type Nameplate = "full" | "compact" | "none";

const NAMEPLATE_KEY = "stream_nameplate";

const NAMEPLATES: Nameplate[] = ["full", "compact", "none"];

export function nameplate(): Nameplate {
  try {
    const saved = localStorage.getItem(NAMEPLATE_KEY);
    return NAMEPLATES.find((mode) => mode === saved) ?? "full";
  } catch {
    return "full";
  }
}

export function setNameplate(mode: Nameplate) {
  try {
    localStorage.setItem(NAMEPLATE_KEY, mode);
  } catch {}
}
