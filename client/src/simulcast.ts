import { gateway } from "./gateway";
import { OpScreenAnswer, OpScreenIce, OpScreenPublish } from "./types/events.gen";
import type { ICECandidate, ScreenPublish } from "./types/events.gen";

const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

export const LAYERS: RTCRtpEncodingParameters[] = [
  { rid: "full", maxBitrate: 1_200_000 },
  { rid: "half", maxBitrate: 300_000, scaleResolutionDownBy: 2 },
];

class ScreenPublisher {
  private pc: RTCPeerConnection | null = null;
  private unsubscribe: Array<() => void> = [];

  async publish(stream: MediaStream): Promise<void> {
    await this.stop();

    const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
    this.pc = pc;

    const video = stream.getVideoTracks()[0];
    if (video) {
      pc.addTransceiver(video, {
        direction: "sendonly",
        streams: [stream],
        sendEncodings: LAYERS,
      });
    }

    const audio = stream.getAudioTracks()[0];
    if (audio) {
      pc.addTransceiver(audio, { direction: "sendonly", streams: [stream] });
    }

    pc.onicecandidate = (event) => {
      if (!event.candidate) return;
      gateway.sendRaw({
        op: OpScreenIce,
        d: {
          candidate: event.candidate.candidate,
          sdp_mid: event.candidate.sdpMid ?? undefined,
          sdp_mline_index: event.candidate.sdpMLineIndex ?? undefined,
        } satisfies ICECandidate,
      });
    };

    this.unsubscribe.push(
      gateway.onControl(OpScreenAnswer, async (payload) => {
        const answer = payload as ScreenPublish;
        if (!this.pc) return;
        await this.pc.setRemoteDescription({ type: "answer", sdp: answer.sdp });
      }),
      gateway.onControl(OpScreenIce, async (payload) => {
        const candidate = payload as ICECandidate;
        if (!this.pc) return;
        await this.pc.addIceCandidate({
          candidate: candidate.candidate,
          sdpMid: candidate.sdp_mid ?? undefined,
          sdpMLineIndex: candidate.sdp_mline_index ?? undefined,
        });
      }),
    );

    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    gateway.sendRaw({ op: OpScreenPublish, d: { sdp: offer.sdp ?? "" } satisfies ScreenPublish });

  }

  async stop() {
    for (const off of this.unsubscribe) off();
    this.unsubscribe = [];

    if (this.pc) {
      this.pc.close();
      this.pc = null;
      gateway.sendRaw({ op: OpScreenPublish, d: { sdp: "" } satisfies ScreenPublish });
    }
  }
}

export const screenPublisher = new ScreenPublisher();
