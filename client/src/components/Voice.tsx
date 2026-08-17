import { useEffect, useState } from "react";
import { gateway } from "../gateway";
import { voice, type VoiceStatus } from "../voice";
import type { Channel, VoiceStateUpdate } from "../types/events.gen";

export function Voice({ channel, selfID }: { channel: Channel; selfID: string }) {
  const [status, setStatus] = useState<VoiceStatus>("idle");
  const [activeChannel, setActiveChannel] = useState<string | null>(null);
  const [members, setMembers] = useState<string[]>([]);
  const [muted, setMuted] = useState(false);

  useEffect(() => voice.onStatusChange((s, id) => {
    setStatus(s);
    setActiveChannel(id);
  }), []);

  useEffect(() => {
    setMembers([]);
    return gateway.on("VOICE_STATE_UPDATE", (payload) => {
      const state = payload as VoiceStateUpdate;
      setMembers((prev) => {
        const without = prev.filter((id) => id !== state.user_id);
        if (state.channel_id === channel.id) return [...without, state.user_id];
        return without;
      });
    });
  }, [channel.id]);

  const here = activeChannel === channel.id;

  return (
    <div className="chat">
      <header className="chat-header">
        <strong>🔊 {channel.name}</strong>
        {here && <span className="muted"> — {status}</span>}
      </header>

      <div className="voice-panel">
        <div className="voice-members">
          {members.length === 0 && <div className="muted">Nobody is here yet.</div>}
          {members.map((id) => (
            <div key={id} className="voice-member">
              <span className="dot ready" />
              {id === selfID ? "You" : id.slice(0, 8)}
            </div>
          ))}
        </div>

        <div className="voice-controls">
          {here ? (
            <>
              <button onClick={() => setMuted(voice.toggleMute())}>
                {muted ? "Unmute" : "Mute"}
              </button>
              <button className="leave" onClick={() => void voice.leave()}>
                Disconnect
              </button>
            </>
          ) : (
            <button onClick={() => void voice.join(channel.id)}>Join voice</button>
          )}
        </div>

        {status === "failed" && (
          <div className="error">
            Could not connect. Check that the microphone permission was granted.
          </div>
        )}
      </div>
    </div>
  );
}
