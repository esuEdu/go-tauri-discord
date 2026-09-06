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

export type OverlayMode = "full" | "compact" | "none";

const OVERLAY_KEY = "call_overlay";

const OVERLAY_MODES: OverlayMode[] = ["full", "compact", "none"];

export function overlayMode(): OverlayMode {
  try {
    const saved = localStorage.getItem(OVERLAY_KEY);
    return OVERLAY_MODES.find((mode) => mode === saved) ?? "full";
  } catch {
    return "full";
  }
}

export function setOverlayMode(mode: OverlayMode) {
  try {
    localStorage.setItem(OVERLAY_KEY, mode);
  } catch {}
}

export type OverlayCorner = "top-left" | "top-right" | "bottom-left" | "bottom-right";

const CORNER_KEY = "call_overlay_corner";

const CORNERS: OverlayCorner[] = ["top-left", "top-right", "bottom-left", "bottom-right"];

export function overlayCorner(): OverlayCorner {
  try {
    const saved = localStorage.getItem(CORNER_KEY);
    return CORNERS.find((corner) => corner === saved) ?? "bottom-right";
  } catch {
    return "bottom-right";
  }
}

export function setOverlayCorner(corner: OverlayCorner) {
  try {
    localStorage.setItem(CORNER_KEY, corner);
  } catch {}
}
