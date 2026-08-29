import { loadRnnoise, RnnoiseWorkletNode } from "@sapphi-red/web-noise-suppressor";
import workletURL from "@sapphi-red/web-noise-suppressor/rnnoiseWorklet.js?url";
import wasmURL from "@sapphi-red/web-noise-suppressor/rnnoise.wasm?url";
import simdURL from "@sapphi-red/web-noise-suppressor/rnnoise_simd.wasm?url";

export const RNNOISE_RATE = 48000;

export type Cleaned = {
  track: MediaStreamTrack;
  source: MediaStreamAudioSourceNode;
  tap: AudioNode;
  stop: () => void;
};

let binary: Promise<ArrayBuffer> | null = null;
let registered: Promise<void> | null = null;

async function ready(context: AudioContext): Promise<ArrayBuffer> {
  binary ??= loadRnnoise({ url: wasmURL, simdUrl: simdURL });
  registered ??= context.audioWorklet.addModule(workletURL);
  const [loaded] = await Promise.all([binary, registered]);
  return loaded;
}

export async function clean(
  context: AudioContext,
  microphone: MediaStream,
): Promise<Cleaned | null> {
  if (context.sampleRate !== RNNOISE_RATE) return null;
  if (!context.audioWorklet) return null;

  let wasmBinary: ArrayBuffer;
  try {
    wasmBinary = await ready(context);
  } catch {
    registered = null;
    return null;
  }

  let node: RnnoiseWorkletNode;
  try {
    node = new RnnoiseWorkletNode(context, { maxChannels: 1, wasmBinary });
  } catch {
    return null;
  }

  const source = context.createMediaStreamSource(microphone);
  const destination = context.createMediaStreamDestination();
  source.connect(node);
  node.connect(destination);

  const track = destination.stream.getAudioTracks()[0];
  if (!track) {
    source.disconnect();
    node.disconnect();
    node.destroy();
    return null;
  }

  return {
    track,
    source,
    tap: node,
    stop: () => {
      source.disconnect();
      node.disconnect();
      node.destroy();
      for (const t of destination.stream.getTracks()) t.stop();
    },
  };
}
