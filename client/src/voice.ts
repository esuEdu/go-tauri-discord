import { gateway } from "./gateway";
import { screenPublisher } from "./simulcast";
import {
  EventVoiceScreenUpdate,
  OpVoiceAnswer,
  OpVoiceCandidate,
  OpVoiceMute,
  OpVoiceOffer,
  OpVoiceScreen,
  OpVoiceState,
  OpVoiceWatch,
  type ICECandidate,
  type SessionDescription,
  type VoiceMuteRequest,
  type VoiceScreenRequest,
  type VoiceWatchRequest,
  type VoiceScreenUpdate,
} from "./types/events.gen";

export type VoiceStatus = "idle" | "connecting" | "connected" | "failed";

export type RemoteScreen = { userID: string | null; stream: MediaStream };

export type ScreenSize = "full" | "half";

export type TrackSource = "mic" | "screen" | "screenaudio";

export type TrackOwner = { source: TrackSource; userID: string };

const SOURCES: TrackSource[] = ["mic", "screen", "screenaudio"];

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function parseTrackName(name: string): TrackOwner | null {
  const first = name.indexOf("-");
  const last = name.lastIndexOf("-");
  if (first < 0 || last <= first) return null;

  const source = SOURCES.find((s) => s === name.slice(0, first));
  const userID = name.slice(first + 1, last);
  if (!source || !UUID.test(userID)) return null;

  return { source, userID };
}

export const MAX_VOLUME = 2;

export type VolumeTarget = "voice" | "screen";

export type Volumes = Record<VolumeTarget, Record<string, number>>;

export const noVolumes: Volumes = { voice: {}, screen: {} };

function targetOf(source: TrackSource): VolumeTarget {
  return source === "screenaudio" ? "screen" : "voice";
}

export type ScreenQualityID = "light" | "smooth" | "sharp" | "high";

export type ScreenQuality = {
  id: ScreenQualityID;
  label: string;
  width: number;
  height: number;
  frameRate: number;
  maxBitrate: number;
  contentHint: "motion" | "detail";
  degradation: RTCDegradationPreference;
};

export const SCREEN_QUALITIES: ScreenQuality[] = [
  {
    id: "light",
    label: "Light — 720p 30fps",
    width: 1280,
    height: 720,
    frameRate: 30,
    maxBitrate: 1_200_000,
    contentHint: "detail",
    degradation: "maintain-resolution",
  },
  {
    id: "smooth",
    label: "Smooth — 720p 60fps",
    width: 1280,
    height: 720,
    frameRate: 60,
    maxBitrate: 3_000_000,
    contentHint: "motion",
    degradation: "maintain-framerate",
  },
  {
    id: "sharp",
    label: "Sharp — 1080p 30fps",
    width: 1920,
    height: 1080,
    frameRate: 30,
    maxBitrate: 4_000_000,
    contentHint: "detail",
    degradation: "maintain-resolution",
  },
  {
    id: "high",
    label: "High — 1080p 60fps",
    width: 1920,
    height: 1080,
    frameRate: 60,
    maxBitrate: 8_000_000,
    contentHint: "detail",
    degradation: "maintain-resolution",
  },
];

export type ScreenState = {
  sharing: boolean;
  sound: boolean;
  local: MediaStream | null;
  remote: RemoteScreen[];
  audible: string[];
  dropped: string[];
  sizes: Record<string, ScreenSize>;
  quality: ScreenQualityID;
};

const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

const QUALITY_KEY = "screen_quality";
const DEFAULT_QUALITY: ScreenQualityID = "smooth";
const VOLUME_KEY = "voice_volumes";

function storedQuality(): ScreenQuality {
  const saved = localStorage.getItem(QUALITY_KEY);
  return SCREEN_QUALITIES.find((q) => q.id === saved) ?? qualityOf(DEFAULT_QUALITY);
}

function qualityOf(id: ScreenQualityID): ScreenQuality {
  return SCREEN_QUALITIES.find((q) => q.id === id) ?? SCREEN_QUALITIES[1];
}

function usableLevels(saved: unknown): Record<string, number> {
  if (typeof saved !== "object" || saved === null) return {};
  return Object.fromEntries(
    Object.entries(saved).filter(
      ([, level]) => typeof level === "number" && level >= 0 && level <= MAX_VOLUME,
    ),
  ) as Record<string, number>;
}

function storedVolumes(): Volumes {
  try {
    const saved = JSON.parse(localStorage.getItem(VOLUME_KEY) ?? "{}") as Record<string, unknown>;
    if (saved.voice === undefined && saved.screen === undefined) {
      return { voice: usableLevels(saved), screen: {} };
    }
    return { voice: usableLevels(saved.voice), screen: usableLevels(saved.screen) };
  } catch {
    return { voice: {}, screen: {} };
  }
}

type Output = {
  audio: HTMLAudioElement;
  node: MediaStreamAudioSourceNode | null;
  gain: GainNode | null;
  userID: string | null;
  target: VolumeTarget;
};

export type Speaking = Record<string, boolean>;

const SPEAKING_LEVEL = 0.02;
const SPEAKING_HOLD = 250;
const METER_INTERVAL = 100;

type Meter = {
  origin: AudioNode;
  analyser: AnalyserNode;
  samples: Uint8Array<ArrayBuffer>;
  loudUntil: number;
};

class VoiceClient {
  private pc: RTCPeerConnection | null = null;
  private microphone: MediaStream | null = null;
  private mixer: AudioContext | null = null;
  private outputs = new Map<string, Output>();
  private volumes = storedVolumes();
  private volumeListeners = new Set<(v: Volumes) => void>();
  private meters = new Map<string, Meter>();
  private meterTimer: ReturnType<typeof setInterval> | null = null;
  private speaking: Speaking = {};
  private speakingListeners = new Set<(s: Speaking) => void>();
  private unsubscribe: Array<() => void> = [];

  private status: VoiceStatus = "idle";
  private channelID: string | null = null;
  private statusListeners = new Set<(s: VoiceStatus, channelID: string | null) => void>();

  private display: MediaStream | null = null;
  private quality: ScreenQuality = storedQuality();
  private videoStreams = new Map<string, MediaStream>();
  private owners = new Map<string, string>();
  private dropped = new Set<string>();
  private sizes: Record<string, ScreenSize> = {};
  private screenListeners = new Set<(s: ScreenState) => void>();

  onStatusChange(fn: (s: VoiceStatus, channelID: string | null) => void): () => void {
    this.statusListeners.add(fn);
    fn(this.status, this.channelID);
    return () => this.statusListeners.delete(fn);
  }

  private setStatus(status: VoiceStatus) {
    this.status = status;
    for (const fn of this.statusListeners) fn(status, this.channelID);
  }

  onScreenChange(fn: (s: ScreenState) => void): () => void {
    this.screenListeners.add(fn);
    fn(this.screens());
    return () => this.screenListeners.delete(fn);
  }

  private screens(): ScreenState {
    const remote: RemoteScreen[] = [];
    for (const [id, stream] of this.videoStreams) {
      remote.push({ userID: this.owners.get(id) ?? parseTrackName(id)?.userID ?? null, stream });
    }
    return {
      sharing: this.display !== null,
      sound: (this.display?.getAudioTracks().length ?? 0) > 0,
      local: this.display,
      remote,
      audible: this.audibleShares(),
      dropped: [...this.dropped],
      sizes: this.sizes,
      quality: this.quality.id,
    };
  }

  watchScreen(userID: string, watching: boolean, size?: ScreenSize) {
    if (watching) {
      this.dropped.delete(userID);
      if (size) this.sizes = { ...this.sizes, [userID]: size };
    } else {
      this.dropped.add(userID);
    }
    gateway.sendRaw({
      op: OpVoiceWatch,
      d: { user_id: userID, watching, size: size ?? "" } satisfies VoiceWatchRequest,
    });
    this.emitScreens();
  }

  sizeOf(userID: string): ScreenSize {
    return this.sizes[userID] ?? "full";
  }

  private audibleShares(): string[] {
    const owners: string[] = [];
    for (const output of this.outputs.values()) {
      if (output.target === "screen" && output.userID && !owners.includes(output.userID)) {
        owners.push(output.userID);
      }
    }
    return owners;
  }

  private emitScreens() {
    const state = this.screens();
    for (const fn of this.screenListeners) fn(state);
  }

  onVolumeChange(fn: (v: Volumes) => void): () => void {
    this.volumeListeners.add(fn);
    fn(this.volumes);
    return () => this.volumeListeners.delete(fn);
  }

  volumeOf(userID: string, target: VolumeTarget): number {
    return this.volumes[target][userID] ?? 1;
  }

  setVolume(userID: string, target: VolumeTarget, level: number) {
    this.volumes = {
      ...this.volumes,
      [target]: {
        ...this.volumes[target],
        [userID]: Math.min(MAX_VOLUME, Math.max(0, level)),
      },
    };
    localStorage.setItem(VOLUME_KEY, JSON.stringify(this.volumes));

    for (const output of this.outputs.values()) {
      if (output.userID === userID && output.target === target) this.applyVolume(output);
    }
    for (const fn of this.volumeListeners) fn(this.volumes);
  }

  private applyVolume(output: Output) {
    const level = output.userID ? this.volumeOf(output.userID, output.target) : 1;
    if (output.gain) {
      output.gain.gain.value = level;
      return;
    }
    output.audio.volume = Math.min(1, level);
  }

  onSpeakingChange(fn: (s: Speaking) => void): () => void {
    this.speakingListeners.add(fn);
    fn(this.speaking);
    return () => this.speakingListeners.delete(fn);
  }

  private watchLevel(userID: string, context: AudioContext, origin: AudioNode) {
    const analyser = context.createAnalyser();
    analyser.fftSize = 512;
    origin.connect(analyser);

    this.meters.set(userID, {
      origin,
      analyser,
      samples: new Uint8Array(analyser.fftSize),
      loudUntil: 0,
    });

    if (this.meterTimer === null) {
      this.meterTimer = setInterval(() => this.measure(), METER_INTERVAL);
    }
  }

  private forgetLevel(userID: string) {
    const meter = this.meters.get(userID);
    if (!meter) return;

    meter.origin.disconnect(meter.analyser);
    this.meters.delete(userID);
    if (this.speaking[userID]) {
      this.speaking = { ...this.speaking, [userID]: false };
      for (const fn of this.speakingListeners) fn(this.speaking);
    }
  }

  private measure() {
    const now = Date.now();
    const next = { ...this.speaking };
    let changed = false;

    for (const [userID, meter] of this.meters) {
      meter.analyser.getByteTimeDomainData(meter.samples);

      let sum = 0;
      for (const sample of meter.samples) {
        const offset = (sample - 128) / 128;
        sum += offset * offset;
      }
      if (Math.sqrt(sum / meter.samples.length) >= SPEAKING_LEVEL) {
        meter.loudUntil = now + SPEAKING_HOLD;
      }

      const loud = now < meter.loudUntil;
      if (next[userID] !== loud) {
        next[userID] = loud;
        changed = true;
      }
    }

    if (!changed) return;
    this.speaking = next;
    for (const fn of this.speakingListeners) fn(next);
  }

  private context(): AudioContext | null {
    if (!this.mixer) {
      try {
        this.mixer = new AudioContext();
      } catch {
        return null;
      }
    }
    void this.mixer.resume().catch(() => undefined);
    return this.mixer;
  }

  get muted(): boolean {
    const track = this.microphone?.getAudioTracks()[0];
    return track ? !track.enabled : false;
  }

  toggleMute(): boolean {
    const track = this.microphone?.getAudioTracks()[0];
    if (!track) return false;

    track.enabled = !track.enabled;
    const muted = !track.enabled;
    gateway.sendRaw({ op: OpVoiceMute, d: { self_mute: muted } satisfies VoiceMuteRequest });
    return muted;
  }

  async join(channelID: string, selfID: string) {
    await this.leave();

    this.channelID = channelID;
    this.setStatus("connecting");

    try {
      this.microphone = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
    } catch {
      this.channelID = null;
      this.setStatus("failed");
      return;
    }

    const mixer = this.context();
    if (mixer) {
      try {
        this.watchLevel(selfID, mixer, mixer.createMediaStreamSource(this.microphone));
      } catch {
        this.forgetLevel(selfID);
      }
    }

    const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
    this.pc = pc;

    for (const track of this.microphone.getAudioTracks()) {
      pc.addTrack(track, this.microphone);
    }

    pc.ontrack = (event) => this.attachRemote(event);

    pc.onicecandidate = (event) => {
      if (!event.candidate) return;
      gateway.sendRaw({
        op: OpVoiceCandidate,
        d: {
          candidate: event.candidate.candidate,
          sdp_mid: event.candidate.sdpMid ?? undefined,
          sdp_mline_index: event.candidate.sdpMLineIndex ?? undefined,
          username_fragment: event.candidate.usernameFragment ?? undefined,
        } satisfies ICECandidate,
      });
    };

    pc.onconnectionstatechange = () => {
      if (pc !== this.pc) return;
      if (pc.connectionState === "connected") this.setStatus("connected");
      if (pc.connectionState === "failed") this.setStatus("failed");
    };

    this.unsubscribe.push(
      gateway.onControl(OpVoiceOffer, async (payload) => {
        const offer = payload as SessionDescription;
        if (!this.pc) return;
        await this.pc.setRemoteDescription({ type: "offer", sdp: offer.sdp });
        const answer = await this.pc.createAnswer();
        await this.pc.setLocalDescription(answer);
        gateway.sendRaw({
          op: OpVoiceAnswer,
          d: { type: "answer", sdp: answer.sdp ?? "" } satisfies SessionDescription,
        });
      }),
      gateway.onControl(OpVoiceCandidate, async (payload) => {
        const candidate = payload as ICECandidate;
        if (!this.pc) return;
        await this.pc.addIceCandidate({
          candidate: candidate.candidate,
          sdpMid: candidate.sdp_mid ?? undefined,
          sdpMLineIndex: candidate.sdp_mline_index ?? undefined,
        });
      }),
      gateway.on(EventVoiceScreenUpdate, (payload) => {
        const update = payload as VoiceScreenUpdate;
        if (update.channel_id !== this.channelID) return;
        if (update.active) {
          this.owners.set(update.stream_id, update.user_id);
        } else {
          this.owners.delete(update.stream_id);
          this.videoStreams.delete(update.stream_id);
        }
        this.emitScreens();
      }),
    );

    gateway.sendRaw({ op: OpVoiceState, d: { channel_id: channelID, self_mute: false, self_deaf: false } });
  }

  async startScreenShare(): Promise<boolean> {
    if (!this.pc) return false;
    if (this.display) return true;

    const capture = (audio: boolean) =>
      navigator.mediaDevices.getDisplayMedia({
        video: this.captureConstraints(),
        audio,
        systemAudio: "include",
      } as DisplayMediaStreamOptions);

    let stream: MediaStream;
    try {
      stream = await capture(true);
    } catch (error) {
      if ((error as DOMException | undefined)?.name === "NotAllowedError") return false;
      try {
        stream = await capture(false);
      } catch {
        return false;
      }
    }

    const track = stream.getVideoTracks()[0];
    if (!track) {
      stream.getTracks().forEach((t) => t.stop());
      return false;
    }

    track.contentHint = this.quality.contentHint;
    track.onended = () => void this.stopScreenShare();

    this.display = stream;
    this.announceScreen(true);
    this.emitScreens();

    void screenPublisher
      .publish(stream, this.quality.maxBitrate, this.quality.degradation)
      .catch(() => undefined);

    return true;
  }

  async stopScreenShare() {
    if (!this.display) return;

    this.display.getTracks().forEach((t) => t.stop());
    this.display = null;
    void screenPublisher.stop();
    this.announceScreen(false);
    this.emitScreens();
  }

  private announceScreen(active: boolean) {
    gateway.sendRaw({ op: OpVoiceScreen, d: { active } satisfies VoiceScreenRequest });
  }





  private captureConstraints(): MediaTrackConstraints {
    const { width, height, frameRate } = this.quality;
    return {
      frameRate: { ideal: frameRate, max: frameRate },
      width: { max: width },
      height: { max: height },
    };
  }


  setScreenQuality(id: ScreenQualityID) {
    this.quality = qualityOf(id);
    localStorage.setItem(QUALITY_KEY, this.quality.id);

    const track = this.display?.getVideoTracks()[0];
    if (track) {
      track.contentHint = this.quality.contentHint;
      void track.applyConstraints(this.captureConstraints()).catch(() => undefined);
      void screenPublisher.retune(this.quality.maxBitrate, this.quality.degradation);
    }
    this.emitScreens();
  }

  private attachRemote(event: RTCTrackEvent) {
    if (event.track.kind === "video") {
      const key = event.streams[0]?.id ?? event.track.id;
      const stream = event.streams[0] ?? new MediaStream([event.track]);
      this.videoStreams.set(key, stream);
      this.emitScreens();
      const forget = () => {
        this.videoStreams.delete(key);
        this.owners.delete(key);
        this.emitScreens();
      };
      event.track.onended = forget;
      event.streams[0]?.addEventListener("removetrack", forget);
      return;
    }

    const [stream] = event.streams;
    if (!stream) return;

    const existing = this.outputs.get(stream.id);
    if (existing) {
      existing.audio.srcObject = stream;
      return;
    }

    const audio = new Audio();
    audio.srcObject = stream;
    audio.autoplay = true;

    const owner = parseTrackName(stream.id);
    const output: Output = {
      audio,
      node: null,
      gain: null,
      userID: owner?.userID ?? null,
      target: owner ? targetOf(owner.source) : "voice",
    };

    const context = this.context();
    if (context) {
      try {
        const node = context.createMediaStreamSource(stream);
        const gain = context.createGain();
        node.connect(gain).connect(context.destination);
        output.node = node;
        output.gain = gain;
        audio.muted = true;
        if (output.target === "voice" && output.userID) {
          this.watchLevel(output.userID, context, node);
        }
      } catch {
        output.node = null;
      }
    }

    this.applyVolume(output);
    void audio.play().catch(() => undefined);
    this.outputs.set(stream.id, output);
    if (output.target === "screen") this.emitScreens();

    stream.onremovetrack = () => this.dropOutput(stream.id);
  }

  private dropOutput(id: string) {
    const output = this.outputs.get(id);
    if (!output) return;
    if (output.target === "screen") queueMicrotask(() => this.emitScreens());
    if (output.target === "voice" && output.userID) this.forgetLevel(output.userID);

    output.node?.disconnect();
    output.gain?.disconnect();
    output.audio.srcObject = null;
    this.outputs.delete(id);
  }

  async leave() {
    if (!this.pc && !this.channelID) return;

    gateway.sendRaw({ op: OpVoiceState, d: { channel_id: null, self_mute: false, self_deaf: false } });

    for (const off of this.unsubscribe) off();
    this.unsubscribe = [];

    for (const id of [...this.outputs.keys()]) this.dropOutput(id);

    if (this.meterTimer !== null) clearInterval(this.meterTimer);
    this.meterTimer = null;
    this.meters.clear();
    this.speaking = {};
    for (const fn of this.speakingListeners) fn(this.speaking);

    void this.mixer?.close().catch(() => undefined);
    this.mixer = null;

    this.display?.getTracks().forEach((t) => t.stop());
    this.display = null;
    this.videoStreams.clear();
    this.owners.clear();
    this.dropped.clear();
    this.sizes = {};

    this.microphone?.getTracks().forEach((t) => t.stop());
    this.microphone = null;

    this.pc?.close();
    this.pc = null;

    this.channelID = null;
    this.setStatus("idle");
    this.emitScreens();
  }
}

export const voice = new VoiceClient();
