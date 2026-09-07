import { audioConstraints } from "./audioPrefs";

const SMOOTHING = 0.55;

export type Listening = {
  stop: () => void;
};

export function listenToMicrophone(
  deviceID: string | null,
  onLevel: (level: number) => void,
  onFail: () => void,
): Listening {
  let stopped = false;
  let stream: MediaStream | null = null;
  let context: AudioContext | null = null;
  let frame = 0;

  void (async () => {
    let opened: MediaStream;
    try {
      opened = await navigator.mediaDevices.getUserMedia({
        audio: { ...audioConstraints(), ...(deviceID ? { deviceId: { exact: deviceID } } : {}) },
      });
    } catch {
      if (!stopped) onFail();
      return;
    }

    if (stopped) {
      opened.getTracks().forEach((track) => track.stop());
      return;
    }

    stream = opened;
    try {
      context = new AudioContext();
    } catch {
      onFail();
      return;
    }

    const analyser = context.createAnalyser();
    analyser.fftSize = 512;
    context.createMediaStreamSource(stream).connect(analyser);

    const samples = new Uint8Array(analyser.fftSize);
    let shown = 0;

    const measure = () => {
      analyser.getByteTimeDomainData(samples);

      let sum = 0;
      for (const sample of samples) {
        const offset = (sample - 128) / 128;
        sum += offset * offset;
      }

      const loudness = Math.min(1, Math.sqrt(sum / samples.length) * 3.2);
      shown = shown * SMOOTHING + loudness * (1 - SMOOTHING);
      onLevel(shown);
      frame = requestAnimationFrame(measure);
    };

    frame = requestAnimationFrame(measure);
  })();

  return {
    stop() {
      stopped = true;
      cancelAnimationFrame(frame);
      stream?.getTracks().forEach((track) => track.stop());
      void context?.close().catch(() => undefined);
    },
  };
}
