import { gateway } from "./gateway";
import { OpPresence } from "./types/events.gen";

const IDLE_AFTER_MS = 10 * 60 * 1000;

const ACTIVITY = [
  "pointerdown",
  "pointermove",
  "keydown",
  "wheel",
  "touchstart",
  "focus",
] as const;

let idle = false;
let timer: ReturnType<typeof setTimeout> | null = null;

function report(next: boolean) {
  if (next === idle) return;
  idle = next;
  gateway.sendRaw({ op: OpPresence, d: { idle } });
}

function restart() {
  report(false);
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => report(true), IDLE_AFTER_MS);
}

function visibility() {
  if (document.visibilityState === "hidden") {
    if (timer) clearTimeout(timer);
    report(true);
    return;
  }
  restart();
}

export function watchForIdleness() {
  for (const event of ACTIVITY) {
    window.addEventListener(event, restart, { passive: true });
  }
  document.addEventListener("visibilitychange", visibility);
  restart();

  return () => {
    for (const event of ACTIVITY) window.removeEventListener(event, restart);
    document.removeEventListener("visibilitychange", visibility);
    if (timer) clearTimeout(timer);
    timer = null;
    idle = false;
  };
}
