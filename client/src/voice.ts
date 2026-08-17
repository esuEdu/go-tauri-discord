import { gateway } from "./gateway";
import {
  OpVoiceAnswer,
  OpVoiceCandidate,
  OpVoiceOffer,
  OpVoiceState,
  type ICECandidate,
  type SessionDescription,
} from "./types/events.gen";

export type VoiceStatus = "idle" | "connecting" | "connected" | "failed";

const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

class VoiceClient {
  private pc: RTCPeerConnection | null = null;
  private microphone: MediaStream | null = null;
  private remotes = new Map<string, HTMLAudioElement>();
  private unsubscribe: Array<() => void> = [];

  private status: VoiceStatus = "idle";
  private channelID: string | null = null;
  private statusListeners = new Set<(s: VoiceStatus, channelID: string | null) => void>();

  onStatusChange(fn: (s: VoiceStatus, channelID: string | null) => void): () => void {
    this.statusListeners.add(fn);
    fn(this.status, this.channelID);
    return () => this.statusListeners.delete(fn);
  }

  private setStatus(status: VoiceStatus) {
    this.status = status;
    for (const fn of this.statusListeners) fn(status, this.channelID);
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
    );

    gateway.sendRaw({ op: OpVoiceState, d: { channel_id: channelID, self_mute: false, self_deaf: false } });
  }

  private attachRemote(event: RTCTrackEvent) {
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

    this.microphone?.getTracks().forEach((t) => t.stop());
    this.microphone = null;

    this.pc?.close();
    this.pc = null;

    this.channelID = null;
    this.setStatus("idle");
  }
}

export const voice = new VoiceClient();
