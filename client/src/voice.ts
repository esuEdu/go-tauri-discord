import { gateway } from "./gateway";
import {
  EventVoiceScreenUpdate,
  OpVoiceAnswer,
  OpVoiceCandidate,
  OpVoiceOffer,
  OpVoiceResync,
  OpVoiceState,
  type ICECandidate,
  type SessionDescription,
  type VoiceScreenUpdate,
} from "./types/events.gen";

export type VoiceStatus = "idle" | "connecting" | "connected" | "failed";

export type RemoteScreen = { userID: string | null; stream: MediaStream };

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
    label: "Light — 720p 15fps",
    width: 1280,
    height: 720,
    frameRate: 15,
    maxBitrate: 800_000,
    contentHint: "detail",
    degradation: "maintain-resolution",
  },
  {
    id: "smooth",
    label: "Smooth — 720p 30fps",
    width: 1280,
    height: 720,
    frameRate: 30,
    maxBitrate: 1_500_000,
    contentHint: "motion",
    degradation: "maintain-framerate",
  },
  {
    id: "sharp",
    label: "Sharp — 1080p 15fps",
    width: 1920,
    height: 1080,
    frameRate: 15,
    maxBitrate: 2_500_000,
    contentHint: "detail",
    degradation: "maintain-resolution",
  },
  {
    id: "high",
    label: "High — 1080p 30fps",
    width: 1920,
    height: 1080,
    frameRate: 30,
    maxBitrate: 4_000_000,
    contentHint: "detail",
    degradation: "maintain-resolution",
  },
];

export type ScreenState = {
  sharing: boolean;
  canShare: boolean;
  local: MediaStream | null;
  remote: RemoteScreen[];
  quality: ScreenQualityID;
};

const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

const QUALITY_KEY = "screen_quality";
const DEFAULT_QUALITY: ScreenQualityID = "smooth";

function storedQuality(): ScreenQuality {
  const saved = localStorage.getItem(QUALITY_KEY);
  return SCREEN_QUALITIES.find((q) => q.id === saved) ?? qualityOf(DEFAULT_QUALITY);
}

function qualityOf(id: ScreenQualityID): ScreenQuality {
  return SCREEN_QUALITIES.find((q) => q.id === id) ?? SCREEN_QUALITIES[1];
}

class VoiceClient {
  private pc: RTCPeerConnection | null = null;
  private microphone: MediaStream | null = null;
  private remotes = new Map<string, HTMLAudioElement>();
  private unsubscribe: Array<() => void> = [];

  private status: VoiceStatus = "idle";
  private channelID: string | null = null;
  private statusListeners = new Set<(s: VoiceStatus, channelID: string | null) => void>();

  private display: MediaStream | null = null;
  private quality: ScreenQuality = storedQuality();
  private screenMid: string | null = null;
  private videoStreams = new Map<string, MediaStream>();
  private owners = new Map<string, string>();
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
      remote.push({ userID: this.owners.get(id) ?? null, stream });
    }
    return {
      sharing: this.display !== null,
      canShare: this.screenMid !== null,
      local: this.display,
      remote,
      quality: this.quality.id,
    };
  }

  private emitScreens() {
    const state = this.screens();
    for (const fn of this.screenListeners) fn(state);
  }

  get muted(): boolean {
    const track = this.microphone?.getAudioTracks()[0];
    return track ? !track.enabled : false;
  }

  toggleMute(): boolean {
    const track = this.microphone?.getAudioTracks()[0];
    if (!track) return false;
    track.enabled = !track.enabled;
    return !track.enabled;
  }

  async join(channelID: string) {
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
        const known = this.screenMid;
        this.screenMid = offer.screen_mid ?? null;
        this.applyScreen();
        if (known !== this.screenMid) this.emitScreens();
        const answer = await this.pc.createAnswer();
        await this.pc.setLocalDescription(answer);
        this.tuneScreen();
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
        if (update.active) this.owners.set(update.stream_id, update.user_id);
        else this.owners.delete(update.stream_id);
        this.emitScreens();
      }),
    );

    gateway.sendRaw({ op: OpVoiceState, d: { channel_id: channelID, self_mute: false, self_deaf: false } });
  }

  async startScreenShare(): Promise<boolean> {
    if (!this.pc || this.screenMid === null) return false;
    if (this.display) return true;

    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getDisplayMedia({
        video: this.captureConstraints(),
        audio: false,
      });
    } catch {
      return false;
    }

    const track = stream.getVideoTracks()[0];
    if (!track) {
      stream.getTracks().forEach((t) => t.stop());
      return false;
    }

    track.contentHint = this.quality.contentHint;
    track.onended = () => void this.stopScreenShare();

    this.display = stream;
    this.applyScreen();
    gateway.sendRaw({ op: OpVoiceResync });
    this.emitScreens();
    return true;
  }

  async stopScreenShare() {
    if (!this.display) return;

    this.display.getTracks().forEach((t) => t.stop());
    this.display = null;
    this.applyScreen();
    gateway.sendRaw({ op: OpVoiceResync });
    this.emitScreens();
  }

  private screenTransceiver(): RTCRtpTransceiver | null {
    if (!this.pc || this.screenMid === null) return null;
    return this.pc.getTransceivers().find((t) => t.mid === this.screenMid) ?? null;
  }

  private applyScreen() {
    const transceiver = this.screenTransceiver();
    if (!transceiver) return;

    const track = this.display?.getVideoTracks()[0] ?? null;
    if (transceiver.sender.track !== track) {
      void transceiver.sender.replaceTrack(track);
    }
    if (this.display && transceiver.sender.setStreams) {
      transceiver.sender.setStreams(this.display);
    }
    if (transceiver.direction !== "sendonly") {
      transceiver.direction = "sendonly";
    }
  }

  private captureConstraints(): MediaTrackConstraints {
    const { width, height, frameRate } = this.quality;
    return {
      frameRate: { ideal: frameRate, max: frameRate },
      width: { max: width },
      height: { max: height },
    };
  }

  private tuneScreen() {
    const sender = this.screenTransceiver()?.sender;
    if (!sender?.track) return;

    const parameters = sender.getParameters();
    if (!parameters.encodings?.length) return;

    parameters.degradationPreference = this.quality.degradation;
    for (const encoding of parameters.encodings) {
      encoding.maxBitrate = this.quality.maxBitrate;
      encoding.maxFramerate = this.quality.frameRate;
    }
    void sender.setParameters(parameters).catch(() => undefined);
  }

  setScreenQuality(id: ScreenQualityID) {
    this.quality = qualityOf(id);
    localStorage.setItem(QUALITY_KEY, this.quality.id);

    const track = this.display?.getVideoTracks()[0];
    if (track) {
      track.contentHint = this.quality.contentHint;
      void track.applyConstraints(this.captureConstraints()).catch(() => undefined);
      this.tuneScreen();
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

    const existing = this.remotes.get(stream.id);
    if (existing) {
      existing.srcObject = stream;
      return;
    }

    const audio = new Audio();
    audio.srcObject = stream;
    audio.autoplay = true;
    void audio.play().catch(() => undefined);
    this.remotes.set(stream.id, audio);

    stream.onremovetrack = () => {
      const element = this.remotes.get(stream.id);
      if (!element) return;
      element.srcObject = null;
      this.remotes.delete(stream.id);
    };
  }

  async leave() {
    if (!this.pc && !this.channelID) return;

    gateway.sendRaw({ op: OpVoiceState, d: { channel_id: null, self_mute: false, self_deaf: false } });

    for (const off of this.unsubscribe) off();
    this.unsubscribe = [];

    for (const audio of this.remotes.values()) {
      audio.srcObject = null;
    }
    this.remotes.clear();

    this.display?.getTracks().forEach((t) => t.stop());
    this.display = null;
    this.screenMid = null;
    this.videoStreams.clear();
    this.owners.clear();

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
