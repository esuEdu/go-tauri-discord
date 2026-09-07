import { soundsOn } from "./audioPrefs";

export type Cue =
  | "joined"
  | "left"
  | "somebodyJoined"
  | "somebodyLeft"
  | "muted"
  | "unmuted"
  | "deafened"
  | "undeafened";

type Shape = {
  notes: number[];
  step: number;
  hold: number;
  level: number;
};

const SHAPES: Record<Cue, Shape> = {
  joined: { notes: [587.33, 880], step: 0.08, hold: 0.16, level: 0.09 },
  left: { notes: [880, 587.33], step: 0.08, hold: 0.16, level: 0.09 },
  somebodyJoined: { notes: [659.25, 987.77], step: 0.06, hold: 0.11, level: 0.05 },
  somebodyLeft: { notes: [987.77, 659.25], step: 0.06, hold: 0.11, level: 0.05 },
  muted: { notes: [440], step: 0, hold: 0.09, level: 0.07 },
  unmuted: { notes: [659.25], step: 0, hold: 0.09, level: 0.07 },
  deafened: { notes: [392, 293.66], step: 0.06, hold: 0.12, level: 0.07 },
  undeafened: { notes: [293.66, 392], step: 0.06, hold: 0.12, level: 0.07 },
};

let piano: AudioContext | null = null;

function wake(): AudioContext | null {
  if (!piano) {
    try {
      piano = new AudioContext();
    } catch {
      return null;
    }
  }
  void piano.resume().catch(() => undefined);
  return piano;
}

export function play(cue: Cue) {
  if (!soundsOn()) return;

  const context = wake();
  if (!context) return;

  const shape = SHAPES[cue];
  const start = context.currentTime;

  shape.notes.forEach((note, index) => {
    const at = start + index * shape.step;
    const tone = context.createOscillator();
    const level = context.createGain();

    tone.type = "sine";
    tone.frequency.setValueAtTime(note, at);

    level.gain.setValueAtTime(0, at);
    level.gain.linearRampToValueAtTime(shape.level, at + 0.012);
    level.gain.exponentialRampToValueAtTime(0.0001, at + shape.hold);

    tone.connect(level).connect(context.destination);
    tone.start(at);
    tone.stop(at + shape.hold + 0.02);
  });
}
