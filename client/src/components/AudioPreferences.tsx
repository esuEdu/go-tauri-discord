import { useEffect, useState } from "react";
import {
  chooseMicrophone,
  chosenMicrophone,
  joinsMuted,
  microphones,
  setJoinsMuted,
  type Microphone,
} from "../audioPrefs";

export function AudioPreferences() {
  const [open, setOpen] = useState(false);
  const [devices, setDevices] = useState<Microphone[]>([]);
  const [chosen, setChosen] = useState<string>(chosenMicrophone() ?? "");
  const [muted, setMuted] = useState(joinsMuted());

  useEffect(() => {
    if (!open) return;
    let live = true;
    void microphones().then((found) => {
      if (live) setDevices(found);
    });
    return () => {
      live = false;
    };
  }, [open]);

  return (
    <div className="audio-prefs">
      <button
        className="secondary"
        aria-expanded={open}
        title="Microphone and joining"
        onClick={() => setOpen(!open)}
      >
        Audio
      </button>

      {open && (
        <div className="audio-prefs-panel">
          <label>
            Microphone
            <select
              value={chosen}
              onChange={(e) => {
                const id = e.target.value;
                setChosen(id);
                chooseMicrophone(id === "" ? null : id);
              }}
            >
              <option value="">Whatever the system picks</option>
              {devices.map((device) => (
                <option key={device.id} value={device.id}>
                  {device.label}
                </option>
              ))}
            </select>
          </label>

          {devices.length === 0 && (
            <div className="muted">
              Microphones are only named once you have joined a call at least once.
            </div>
          )}

          <label className="audio-prefs-check">
            <input
              type="checkbox"
              checked={muted}
              onChange={(e) => {
                setMuted(e.target.checked);
                setJoinsMuted(e.target.checked);
              }}
            />
            Join muted
          </label>

          <div className="muted">A change applies the next time you join.</div>
        </div>
      )}
    </div>
  );
}
