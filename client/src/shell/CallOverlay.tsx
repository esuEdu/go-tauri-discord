import { useEffect, useState } from "react";
import { Avatar } from "../ui/Avatar";
import type { OverlayMode } from "../streamPrefs";

export type OverlayPerson = {
  id: string;
  name: string;
  avatarURL: string | null;
  speaking: boolean;
};

export type OverlayState = {
  mode: OverlayMode;
  people: OverlayPerson[];
};

export const OVERLAY_EVENT = "overlay://call";
export const OVERLAY_READY = "overlay://ready";

export function CallOverlay() {
  const [state, setState] = useState<OverlayState>({ mode: "full", people: [] });

  useEffect(() => {
    let stop: (() => void) | undefined;
    let dropped = false;

    void (async () => {
      const { emit, listen } = await import("@tauri-apps/api/event");
      const off = await listen<OverlayState>(OVERLAY_EVENT, (event) => setState(event.payload));
      if (dropped) {
        off();
        return;
      }
      stop = off;
      await emit(OVERLAY_READY).catch(() => undefined);
    })();

    return () => {
      dropped = true;
      stop?.();
    };
  }, []);

  if (state.mode === "none" || state.people.length === 0) return null;

  return (
    <div className="call-overlay" data-size={state.mode}>
      {state.people.map((person) => (
        <div key={person.id} className="call-overlay-row" data-speaking={person.speaking}>
          <Avatar name={person.name} url={person.avatarURL} size={28} />
          {state.mode === "full" && (
            <span className="call-overlay-name">{person.name}</span>
          )}
        </div>
      ))}
    </div>
  );
}
