import { useEffect, useRef, useState } from "react";
import { canRelay } from "../ice";
import {
  voice,
  MAX_VOLUME,
  SCREEN_QUALITIES,
  type ScreenQualityID,
  noVolumes,
  type ScreenState,
  type VoiceStatus,
  type Speaking,
  type VolumeTarget,
  type Volumes,
} from "../voice";
import { AudioPreferences } from "./AudioPreferences";
import { emptySession, session, type SessionState } from "../session";
import { STREAM, allows } from "../permissions";
import type { Channel } from "../types/events.gen";

const emptyScreens: ScreenState = {
  sharing: false,
  sound: false,
  local: null,
  remote: [],
  audible: [],
  dropped: [],
  sizes: {},
  quality: "smooth",
};

function ScreenTile({
  label,
  stream,
  controls,
}: {
  label: string;
  stream: MediaStream;
  controls?: React.ReactNode;
}) {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.srcObject = stream;
    void element.play().catch(() => undefined);
  }, [stream]);

  return (
    <figure className="screen-tile">
      <video ref={ref} autoPlay playsInline muted />
      <figcaption className="muted">
        <span className="channel-name">{label}</span>
        {controls}
      </figcaption>
    </figure>
  );
}

function VolumeSlider({
  userID,
  name,
  target,
  level,
  tagged,
}: {
  userID: string;
  name: string;
  target: VolumeTarget;
  level: number;
  tagged: boolean;
}) {
  const percent = Math.round(level * 100);

  return (
    <label className="volume">
      {tagged && <span className="volume-tag muted">{target}</span>}
      <input
        type="range"
        min={0}
        max={MAX_VOLUME * 100}
        step={5}
        value={percent}
        aria-label={target === "screen" ? `Screen volume for ${name}` : `Volume for ${name}`}
        onChange={(e) => voice.setVolume(userID, target, Number(e.target.value) / 100)}
      />
      <span className="muted">{percent}%</span>
    </label>
  );
}

export function Voice({ channel, selfID }: { channel: Channel; selfID: string }) {
  const [status, setStatus] = useState<VoiceStatus>("idle");
  const [activeChannel, setActiveChannel] = useState<string | null>(null);
  const [muted, setMuted] = useState(false);
  const [deafened, setDeafened] = useState(false);
  const [screens, setScreens] = useState<ScreenState>(emptyScreens);
  const [volumes, setVolumes] = useState<Volumes>(noVolumes);
  const [people, setPeople] = useState<SessionState>(emptySession);
  const [speaking, setSpeaking] = useState<Speaking>({});
  const [pickerRefused, setPickerRefused] = useState(false);

  useEffect(() => session.onChange(setPeople), []);

  useEffect(() => voice.onStatusChange((s, id) => {
    setStatus(s);
    setActiveChannel(id);
    setMuted(voice.muted);
    setDeafened(voice.deaf);
  }), []);

  useEffect(() => voice.onScreenChange(setScreens), []);

  useEffect(() => voice.onVolumeChange(setVolumes), []);

  useEffect(() => voice.onSpeakingChange(setSpeaking), []);


  const here = activeChannel === channel.id;
  const members = people.inVoice[channel.id] ?? [];
  const mutes = people.mutedInVoice;
  const deafs = people.deafenedInVoice;
  const mayShare = allows(people.channelAllows[channel.id], STREAM);

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
            {screens.local && <ScreenTile label="Your screen" stream={screens.local} />}
            {screens.remote.map((screen) => (
              <ScreenTile
                key={screen.stream.id}
                label={screen.userID ? `${session.nameOf(screen.userID)}'s screen` : "A shared screen"}
                stream={screen.stream}
                controls={
                  screen.userID && (
                    <>
                      <button
                        className={screens.sizes[screen.userID] === "half" ? "link" : "link active"}
                        onClick={() => voice.watchScreen(screen.userID as string, true, "full")}
                      >
                        Full
                      </button>
                      <button
                        className={screens.sizes[screen.userID] === "half" ? "link active" : "link"}
                        onClick={() => voice.watchScreen(screen.userID as string, true, "half")}
                      >
                        Smaller
                      </button>
                      <button
                        className="link"
                        onClick={() => voice.watchScreen(screen.userID as string, false)}
                      >
                        Stop
                      </button>
                    </>
                  )
                }
              />
            ))}
          </div>
        )}

        {here && screens.dropped.length > 0 && (
          <div className="dropped-shares">
            {screens.dropped.map((id) => (
              <div key={id} className="dropped-share muted">
                <span className="channel-name">
                  Not watching {session.nameOf(id)}'s screen
                </span>
                <button className="link" onClick={() => voice.watchScreen(id, true)}>
                  Watch again
                </button>
              </div>
            ))}
          </div>
        )}

        <div className="voice-members">
          {members.length === 0 && <div className="muted">Nobody is here yet.</div>}
          {members.map((id) => (
            <div key={id} className={speaking[id] ? "voice-member speaking" : "voice-member"}>
              <span className={people.online[id] ? "dot ready" : "dot closed"} />
              <span className="voice-name">
                {id === selfID ? "You" : people.names[id] ?? id.slice(0, 8)}
              </span>
              {(id === selfID ? muted : mutes[id]) && (
                <span className="muted voice-muted">muted</span>
              )}
              {(id === selfID ? deafened : deafs[id]) && (
                <span className="muted voice-muted">can't hear</span>
              )}
              {people.connection[id] && people.connection[id] !== "good" && (
                <span
                  className={`connection ${people.connection[id]}`}
                  title={
                    people.connection[id] === "poor"
                      ? "Their connection is dropping audio"
                      : "Their connection is struggling"
                  }
                >
                  {people.connection[id] === "poor" ? "bad connection" : "weak connection"}
                </span>
              )}
              {here && id !== selfID && (
                <div className="volumes">
                  <VolumeSlider
                    userID={id}
                    name={people.names[id] ?? id.slice(0, 8)}
                    target="voice"
                    level={volumes.voice[id] ?? 1}
                    tagged={screens.audible.includes(id)}
                  />
                  {screens.audible.includes(id) && (
                    <VolumeSlider
                      userID={id}
                      name={people.names[id] ?? id.slice(0, 8)}
                      target="screen"
                      level={volumes.screen[id] ?? 1}
                      tagged
                    />
                  )}
                </div>
              )}
            </div>
          ))}
        </div>

        <div className="voice-controls">
          {here ? (
            <>
              <button onClick={() => { setMuted(voice.toggleMute()); setDeafened(voice.deaf); }}>
                {muted ? "Unmute" : "Mute"}
              </button>
              <button onClick={() => { setDeafened(voice.toggleDeafen()); setMuted(voice.muted); }}>
                {deafened ? "Undeafen" : "Deafen"}
              </button>
              {screens.sharing ? (
                <button onClick={() => void voice.stopScreenShare()}>
                  {screens.sound ? "Stop sharing (with sound)" : "Stop sharing"}
                </button>
              ) : (
                <button disabled={!mayShare} onClick={() => void share()}>
                  Share screen
                </button>
              )}
              {mayShare && (
                <select
                  className="quality"
                  aria-label="Screen share quality"
                  value={screens.quality}
                  onChange={(e) => voice.setScreenQuality(e.target.value as ScreenQualityID)}
                >
                  {SCREEN_QUALITIES.map((quality) => (
                    <option key={quality.id} value={quality.id}>
                      {quality.label}
                    </option>
                  ))}
                </select>
              )}
              <button className="leave" onClick={() => void voice.leave()}>
                Disconnect
              </button>
              <AudioPreferences />
            </>
          ) : (
            <>
              <button onClick={() => void voice.join(channel.id, selfID)}>Join voice</button>
              <AudioPreferences />
            </>
          )}
        </div>

        {here && screens.sharing && !screens.sound && (
          <div className="muted">
            Sharing without sound. A browser only captures audio from a tab — reshare, pick the
            tab that is playing, and tick “Also share tab audio”.
          </div>
        )}

        {here && !mayShare && status === "connected" && (
          <div className="muted">You do not have permission to share a screen here.</div>
        )}

        {pickerRefused && (
          <div className="muted">
            No screen was picked. Some desktop builds cannot capture a screen — try the browser.
          </div>
        )}

        {status === "failed" && (
          <div className="error">
            {voice.reasonForFailure() === "network"
              ? canRelay()
                ? "Could not reach the voice server. Something on the way blocked the connection."
                : "Could not reach the voice server. This network needs a TURN relay to get out, " +
                  "and this server has none configured."
              : "Could not connect. Check that the microphone permission was granted."}
          </div>
        )}
      </div>
    </div>
  );
}
