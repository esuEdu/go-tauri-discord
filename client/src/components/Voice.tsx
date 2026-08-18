import { useEffect, useRef, useState } from "react";
import { gateway } from "../gateway";
import { voice, type ScreenState, type VoiceStatus } from "../voice";
import type { Channel, VoiceStateUpdate } from "../types/events.gen";

const emptyScreens: ScreenState = { sharing: false, canShare: false, local: null, remote: [] };

function ScreenTile({ label, stream, muted }: { label: string; stream: MediaStream; muted: boolean }) {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.srcObject = stream;
    void element.play().catch(() => undefined);
  }, [stream]);

  return (
    <figure className="screen-tile">
      <video ref={ref} autoPlay playsInline muted={muted} />
      <figcaption className="muted">{label}</figcaption>
    </figure>
  );
}

export function Voice({ channel, selfID }: { channel: Channel; selfID: string }) {
  const [status, setStatus] = useState<VoiceStatus>("idle");
  const [activeChannel, setActiveChannel] = useState<string | null>(null);
  const [members, setMembers] = useState<string[]>([]);
  const [muted, setMuted] = useState(false);
  const [screens, setScreens] = useState<ScreenState>(emptyScreens);
  const [pickerRefused, setPickerRefused] = useState(false);

  useEffect(() => voice.onStatusChange((s, id) => {
    setStatus(s);
    setActiveChannel(id);
  }), []);

  useEffect(() => voice.onScreenChange(setScreens), []);

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

  const share = async () => {
    setPickerRefused(false);
    if (!(await voice.startScreenShare())) setPickerRefused(true);
  };

  return (
    <div className="chat">
      <header className="chat-header">
        <strong>🔊 {channel.name}</strong>
        {here && <span className="muted"> — {status}</span>}
      </header>

      <div className="voice-panel">
        {here && (screens.local || screens.remote.length > 0) && (
          <div className="screen-grid">
            {screens.local && (
              <ScreenTile label="Your screen" stream={screens.local} muted />
            )}
            {screens.remote.map((screen) => (
              <ScreenTile
                key={screen.stream.id}
                label={screen.userID ? `${screen.userID.slice(0, 8)}'s screen` : "A shared screen"}
                stream={screen.stream}
                muted={false}
              />
            ))}
          </div>
        )}

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
              {screens.sharing ? (
                <button onClick={() => void voice.stopScreenShare()}>Stop sharing</button>
              ) : (
                <button disabled={!screens.canShare} onClick={() => void share()}>
                  Share screen
                </button>
              )}
              <button className="leave" onClick={() => void voice.leave()}>
                Disconnect
              </button>
            </>
          ) : (
            <button onClick={() => void voice.join(channel.id)}>Join voice</button>
          )}
        </div>

        {here && !screens.canShare && status === "connected" && (
          <div className="muted">You do not have permission to share a screen here.</div>
        )}

        {pickerRefused && (
          <div className="muted">
            No screen was picked. Some desktop builds cannot capture a screen — try the browser.
          </div>
        )}

        {status === "failed" && (
          <div className="error">
            Could not connect. Check that the microphone permission was granted.
          </div>
        )}
      </div>
    </div>
  );
}
