import { gateway } from "./gateway";
import { iceServers } from "./ice";
import { OpScreenAnswer, OpScreenIce, OpScreenPublish } from "./types/events.gen";
import type { ICECandidate, ScreenPublish } from "./types/events.gen";
import type { ScreenQuality } from "./voice";

type Unlisten = () => void;

async function tauri() {
  const [core, event] = await Promise.all([
    import("@tauri-apps/api/core"),
    import("@tauri-apps/api/event"),
  ]);
  return { invoke: core.invoke, listen: event.listen };
}

function serverIceServers() {
  return iceServers().map((server) => ({
    urls: Array.isArray(server.urls) ? server.urls : [server.urls],
    username: server.username ?? null,
    credential: typeof server.credential === "string" ? server.credential : null,
  }));
}

class ScreenPublisher {
  private off: Unlisten[] = [];

  async publish(sourceID: string, quality: ScreenQuality, audio: boolean): Promise<void> {
    await this.stop();

    const { invoke, listen } = await tauri();

    const offers = await listen<string>("screen://offer", (event) => {
      gateway.sendRaw({
        op: OpScreenPublish,
        d: { sdp: event.payload } satisfies ScreenPublish,
      });
    });

    const ended = await listen("screen://ended", () => {
      void this.stop();
    });

    this.off.push(
      offers,
      ended,
      gateway.onControl(OpScreenAnswer, async (payload) => {
        const answer = payload as ScreenPublish;
        await invoke("screen_answer", { sdp: answer.sdp }).catch(() => undefined);
      }),
      gateway.onControl(OpScreenIce, async (payload) => {
        const candidate = payload as ICECandidate;
        await invoke("screen_candidate", {
          candidate: candidate.candidate,
          sdpMid: candidate.sdp_mid ?? null,
          sdpMlineIndex: candidate.sdp_mline_index ?? null,
        }).catch(() => undefined);
      }),
    );

    try {
      await invoke("start_screen_share", {
        sourceId: sourceID,
        quality: {
          width: quality.width,
          height: quality.height,
          frame_rate: quality.frameRate,
          max_bitrate: quality.maxBitrate,
        },
        audio,
        iceServers: serverIceServers(),
      });
    } catch (error) {
      await this.stop();
      throw error;
    }
  }

  async stop() {
    for (const off of this.off) off();
    this.off = [];

    if (!("__TAURI_INTERNALS__" in window)) return;

    const { invoke } = await tauri();
    await invoke("stop_screen_share").catch(() => undefined);
    gateway.sendRaw({ op: OpScreenPublish, d: { sdp: "" } satisfies ScreenPublish });
  }
}

export const screenPublisher = new ScreenPublisher();
