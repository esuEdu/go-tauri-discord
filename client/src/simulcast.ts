import { gateway } from "./gateway";
import { OpScreenAnswer, OpScreenIce, OpScreenPublish } from "./types/events.gen";
import type { ICECandidate, ScreenPublish } from "./types/events.gen";

const ICE_SERVERS: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

export const LAYERS: RTCRtpEncodingParameters[] = [
  { rid: "full", maxBitrate: 1_200_000 },
  { rid: "half", maxBitrate: 300_000, scaleResolutionDownBy: 2 },
];

export type PublishReport = {
  asked: string[];
  offered: string[];
  negotiated: string[];
  note: string;
};

function ridsInSDP(sdp: string): string[] {
  return [...sdp.matchAll(/^a=rid:([^ ]+) send/gm)].map((m) => m[1]);
}

class ScreenPublisher {
  private pc: RTCPeerConnection | null = null;
  private unsubscribe: Array<() => void> = [];

  async publish(track: MediaStreamTrack, stream: MediaStream): Promise<PublishReport> {
    await this.stop();

    const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
    this.pc = pc;

    pc.addTransceiver(track, {
      direction: "sendonly",
      streams: [stream],
      sendEncodings: LAYERS,
    });

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

    const sender = pc.getTransceivers()[0]?.sender;
    const negotiated = (sender?.getParameters().encodings ?? [])
      .map((e) => e.rid)
      .filter((rid): rid is string => typeof rid === "string");
    const offered = ridsInSDP(offer.sdp ?? "");

    return {
      asked: LAYERS.map((l) => l.rid as string),
      offered,
      negotiated,
      note:
        offered.length > 1
          ? "this browser offers more than one size"
          : "this browser collapsed the sizes into one — simulcast is not available here",
    };
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
