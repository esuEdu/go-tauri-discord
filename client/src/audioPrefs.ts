const MIC_KEY = "microphone_id";
const JOIN_MUTED_KEY = "join_muted";
const SUPPRESS_KEY = "suppress_noise";

export type Microphone = {
  id: string;
  label: string;
};

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {}
}

export function chosenMicrophone(): string | null {
  return read(MIC_KEY);
}

export function chooseMicrophone(deviceID: string | null) {
  write(MIC_KEY, deviceID);
}

export function joinsMuted(): boolean {
  return read(JOIN_MUTED_KEY) === "true";
}

export function setJoinsMuted(muted: boolean) {
  write(JOIN_MUTED_KEY, muted ? "true" : "false");
}

export function suppressesNoise(): boolean {
  return read(SUPPRESS_KEY) !== "false";
}

export function setSuppressesNoise(on: boolean) {
  write(SUPPRESS_KEY, on ? "true" : "false");
}

export function audioConstraints(): MediaTrackConstraints {
  const shared: MediaTrackConstraints = {
    echoCancellation: true,
    noiseSuppression: !suppressesNoise(),
    autoGainControl: true,
  };
  const chosen = chosenMicrophone();
  if (!chosen) return shared;
  return { ...shared, deviceId: { ideal: chosen } };
}

export async function microphones(): Promise<Microphone[]> {
  if (!navigator.mediaDevices?.enumerateDevices) return [];
  try {
    const devices = await navigator.mediaDevices.enumerateDevices();
    return devices
      .filter((device) => device.kind === "audioinput")
      .map((device, at) => ({
        id: device.deviceId,
        label: device.label || `Microphone ${at + 1}`,
      }));
  } catch {
    return [];
  }
}
