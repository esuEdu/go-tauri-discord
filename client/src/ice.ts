import { gateway } from "./gateway";
import type { ICEServer, Ready } from "./types/events.gen";

const FALLBACK: RTCIceServer[] = [{ urls: "stun:stun.l.google.com:19302" }];

let configured: RTCIceServer[] = FALLBACK;

function adopt(servers: ICEServer[]) {
  if (!servers || servers.length === 0) {
    configured = FALLBACK;
    return;
  }
  configured = servers.map((server) => ({
    urls: server.urls,
    ...(server.username ? { username: server.username } : {}),
    ...(server.credential ? { credential: server.credential } : {}),
  }));
}

gateway.on("READY", (payload) => adopt((payload as Ready).ice_servers));

export function iceServers(): RTCIceServer[] {
  return configured;
}

export function canRelay(): boolean {
  return configured.some((server) => {
    const urls = Array.isArray(server.urls) ? server.urls : [server.urls];
    return urls.some((url) => url.startsWith("turn:") || url.startsWith("turns:"));
  });
}
