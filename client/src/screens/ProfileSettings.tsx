import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import {
  chosenMicrophone,
  joinsMuted,
  microphones,
  setJoinsMuted,
  setSoundLevel,
  setSoundsFor,
  setSoundsOn,
  soundLevel,
  soundsFor,
  soundsOn,
  type Microphone,
} from "../audioPrefs";
import { listenToMicrophone } from "../micLevel";
import type { Nameplate } from "../streamPrefs";
import type { User } from "../types/events.gen";
import { checkForUpdate, currentVersion, type Release } from "../updates";
import { Avatar } from "../ui/Avatar";
import { Sheet } from "../ui/Sheet";
import { PlacePicture } from "./PlacePicture";
import { UpdateSheet } from "./UpdatePrompt";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Toggle } from "../ui/Toggle";
import { VolumeSlider } from "../ui/VolumeSlider";
import { play } from "../sounds";
import { voice } from "../voice";

type Tab = "account" | "voice" | "alerts" | "look" | "about";

const TABS: { id: Tab; label: string }[] = [
  { id: "account", label: "Account" },
  { id: "voice", label: "Voice" },
  { id: "alerts", label: "Alerts" },
  { id: "look", label: "Look" },
  { id: "about", label: "About" },
];

export function ProfileSettings({
  user,
  bio,
  avatarURL,
  onClose,
  onChanged,
  onSignOut,
  onDeleteAccount,
  nameplate,
  onNameplate,
}: {
  user: User;
  bio: string | null;
  avatarURL: string | null;
  onClose: () => void;
  onChanged: () => void;
  onSignOut: () => void;
  onDeleteAccount: () => void;
  nameplate: Nameplate;
  onNameplate: (mode: Nameplate) => void;
}) {
  const [tab, setTab] = useState<Tab>("account");

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="scrim" onPointerDown={onClose}>
      <div
        className="profile-settings"
        role="dialog"
        aria-label="Your settings"
        onPointerDown={(event) => event.stopPropagation()}
      >
        <div className="profile-tabs">
          {TABS.map((entry) => (
            <button
              key={entry.id}
              type="button"
              className="profile-tab"
              data-active={entry.id === tab}
              onClick={() => setTab(entry.id)}
            >
              {entry.label}
            </button>
          ))}
        </div>

        <div className="profile-content" data-tab={tab}>
          {tab === "account" && (
            <AccountTab
              user={user}
              bio={bio}
              avatarURL={avatarURL}
              onChanged={onChanged}
              onSignOut={onSignOut}
              onDeleteAccount={onDeleteAccount}
            />
          )}
          {tab === "voice" && <VoiceTab />}
          {tab === "alerts" && <AlertsTab />}
          {tab === "look" && (
            <LookTab nameplate={nameplate} onNameplate={onNameplate} />
          )}
          {tab === "about" && <AboutTab />}
        </div>
      </div>
    </div>
  );
}

function AccountTab({
  user,
  bio,
  avatarURL,
  onChanged,
  onSignOut,
  onDeleteAccount,
}: {
  user: User;
  bio: string | null;
  avatarURL: string | null;
  onChanged: () => void;
  onSignOut: () => void;
  onDeleteAccount: () => void;
}) {
  const [placing, setPlacing] = useState<File | null>(null);
  const [removing, setRemoving] = useState(false);
  const [draft, setDraft] = useState(bio ?? "");
  const [saving, setSaving] = useState(false);

  useEffect(() => setDraft(bio ?? ""), [bio]);

  async function saveBio() {
    setSaving(true);
    try {
      await api.updateProfile({ bio: draft });
      onChanged();
    } finally {
      setSaving(false);
    }
  }

  function pickPicture() {
    const picker = document.createElement("input");
    picker.type = "file";
    picker.accept = "image/*";
    picker.onchange = () => {
      const file = picker.files?.[0];
      if (file) setPlacing(file);
    };
    picker.click();
  }

  return (
    <>
      <div className="profile-picture-row">
        <Avatar name={user.username} url={avatarURL} size={56} tone="accent" />
        <div className="profile-picture-side">
          <div className="profile-picture-buttons">
            <Button onClick={pickPicture}>
              {user.avatar_key ? "Change picture" : "Add a picture"}
            </Button>
            <Button
              kind="quiet"
              disabled={!user.avatar_key}
              onClick={() => setRemoving(true)}
            >
              Remove
            </Button>
          </div>
          <p className="profile-hint">
            5 MB and 24 megapixels at most. The middle is kept, squared, shrunk to 256px.
          </p>
        </div>
      </div>

      <div className="profile-bio-block">
        <label className="profile-field-label" htmlFor="bio">
          About you
        </label>
        <textarea
          id="bio"
          className="profile-bio-input"
          value={draft}
          maxLength={500}
          rows={3}
          placeholder="A line or two, shown to anybody who clicks your name."
          onChange={(event) => setDraft(event.target.value)}
        />
        <div className="profile-bio-foot">
          <span className="profile-hint">{draft.length}/500</span>
          <Button disabled={saving || draft === (bio ?? "")} onClick={saveBio}>
            {saving ? "Saving…" : "Save"}
          </Button>
        </div>
      </div>

      <div className="profile-card">
        <div className="profile-card-row">
          <span className="profile-field">
            <span className="profile-field-label">Name</span>
            <span className="profile-field-value">
              {user.username} <span className="profile-tag">#{user.discriminator}</span>
            </span>
          </span>
          <span className="profile-aside">fixed at sign-up</span>
        </div>
        <div className="profile-card-row">
          <span className="profile-field">
            <span className="profile-field-label">Password</span>
            <span className="profile-field-value">••••••••</span>
          </span>
          <span className="profile-aside">changing it needs the server</span>
        </div>
      </div>

      <div className="profile-actions">
        <button type="button" className="profile-action" onClick={onSignOut}>
          <Icon name="sign-out" size={16} />
          <span className="profile-action-label">Sign out</span>
        </button>
        <button
          type="button"
          className="profile-action"
          data-tone="bad"
          onClick={onDeleteAccount}
        >
          <Icon name="trash" size={16} />
          <span className="profile-action-label">Delete your account</span>
        </button>
      </div>

      {placing && (
        <PlacePicture
          file={placing}
          onCancel={() => setPlacing(null)}
          onUse={async (cropped) => {
            setPlacing(null);
            await api.setAvatar(cropped);
            onChanged();
          }}
        />
      )}

      {removing && (
        <Sheet
          title="Remove your picture"
          subtitle="Everyone sees your initials again. The file is deleted from the server, so putting it back means uploading it once more."
          onClose={() => setRemoving(false)}
        >
          <div className="sheet-actions">
            <Button kind="quiet" onClick={() => setRemoving(false)}>
              Never mind
            </Button>
            <Button
              kind="danger"
              onClick={async () => {
                setRemoving(false);
                await api.clearAvatar();
                onChanged();
              }}
            >
              Remove it
            </Button>
          </div>
        </Sheet>
      )}
    </>
  );
}

function VoiceTab() {
  const [mics, setMics] = useState<Microphone[]>([]);
  const [mic, setMic] = useState<string | null>(chosenMicrophone);
  const [muted, setMuted] = useState(joinsMuted);
  const [sounds, setSounds] = useState(soundsOn);
  const [level, setLevel] = useState(soundLevel);
  const [ownSounds, setOwnSounds] = useState(() => soundsFor("self"));
  const [otherSounds, setOtherSounds] = useState(() => soundsFor("others"));
  const [controlSounds, setControlSounds] = useState(() => soundsFor("controls"));
  const preview = useRef<number | undefined>(undefined);
  const [heard, setHeard] = useState(0);
  const [quiet, setQuiet] = useState<"busy" | "failed" | null>(null);

  useEffect(() => {
    void microphones().then(setMics);

    const devices = navigator.mediaDevices;
    if (!devices?.addEventListener) return;

    const relist = () => void microphones().then(setMics);
    devices.addEventListener("devicechange", relist);
    return () => devices.removeEventListener("devicechange", relist);
  }, []);

  useEffect(() => {
    setQuiet(null);
    setHeard(0);
    const listening = listenToMicrophone(mic, setHeard, () =>
      setQuiet(voice.inCall ? "busy" : "failed"),
    );
    return () => listening.stop();
  }, [mic]);

  useEffect(() => {
    if (mic === null || mics.length === 0) return;
    if (mics.some((entry) => entry.id === mic)) return;

    setMic(null);
    void voice.useMicrophone(null);
  }, [mics, mic]);

  return (
    <>
      <label className="profile-field-block">
        <span className="profile-field-label">Microphone</span>
        <select
          className="profile-select"
          value={mic ?? ""}
          onChange={(event) => {
            const id = event.target.value || null;
            setMic(id);
            void voice.useMicrophone(id);
          }}
        >
          <option value="">System default</option>
          {mics.map((entry) => (
            <option key={entry.id} value={entry.id}>
              {entry.label}
            </option>
          ))}
        </select>
      </label>

      <div className="profile-field-block">
        <span className="profile-field-label">Output</span>
        <span className="profile-reading">
          Whatever your computer is set to
          <span className="profile-reading-why">
            Vocalis does not choose a speaker of its own — change it where you change it
            for everything else
          </span>
        </span>
      </div>

      <div className="profile-level">
        <div className="profile-level-head">
          <span>Say something</span>
          <span>{quiet ? "no signal" : `${Math.round(heard * 100)}%`}</span>
        </div>
        <div className="profile-meter">
          {Array.from({ length: 10 }, (_, i) => (
            <span
              key={i}
              className="profile-meter-bar"
              data-lit={!quiet && heard * 10 > i}
              data-loud={i > 7}
            />
          ))}
        </div>
        <p className="profile-hint">
          {quiet === "busy"
            ? "The call has the microphone, so it cannot be listened to twice. The bars come back when you leave."
            : quiet === "failed"
              ? "Nothing is coming from this microphone. Another program may be holding it, or Vocalis may not be allowed to use it."
              : "This is the microphone itself, before anybody hears it — the way to tell a dead one from a quiet room without joining a call."}
        </p>
      </div>

      <span className="profile-divider" />

      <div className="profile-toggles">
        <div className="profile-toggle-row">
          <Toggle on={false} label="Push to talk instead of always open" onChange={() => {}} />
          <span className="profile-toggle-label">Push to talk instead of always open</span>
        </div>
        <div className="profile-toggle-row">
          <Toggle
            on={muted}
            label="Join calls with the microphone off"
            onChange={(on) => {
              setMuted(on);
              setJoinsMuted(on);
            }}
          />
          <span className="profile-toggle-label">Join calls with the microphone off</span>
        </div>
        <div className="profile-toggle-row">
          <span className="profile-keycap">⌥ M</span>
          <span className="profile-toggle-label">mute, from anywhere</span>
        </div>
      </div>

      <span className="profile-divider" />

      <span className="profile-title">Sounds</span>

      <div className="profile-toggles">
        <div className="profile-toggle-row">
          <Toggle
            on={sounds}
            label="Sounds when a call changes"
            onChange={(on) => {
              setSounds(on);
              setSoundsOn(on);
              if (on) play("joined");
            }}
          />
          <span className="profile-toggle-label">Sounds when a call changes</span>
        </div>

        {sounds && (
          <>
            <div className="profile-sound-level">
              <VolumeSlider
                label="How loud"
                value={level}
                max={1}
                onChange={(next) => {
                  setLevel(next);
                  setSoundLevel(next);
                  window.clearTimeout(preview.current);
                  preview.current = window.setTimeout(() => play("joined"), 220);
                }}
              />
            </div>

            <div className="profile-toggle-row">
              <Toggle
                on={ownSounds}
                label="When you join and leave"
                onChange={(on) => {
                  setOwnSounds(on);
                  setSoundsFor("self", on);
                  if (on) play("joined");
                }}
              />
              <span className="profile-toggle-label">When you join and leave a call</span>
            </div>

            <div className="profile-toggle-row">
              <Toggle
                on={otherSounds}
                label="When others come and go"
                onChange={(on) => {
                  setOtherSounds(on);
                  setSoundsFor("others", on);
                  if (on) play("somebodyJoined");
                }}
              />
              <span className="profile-toggle-label">
                When somebody else comes or goes while you are in a call
              </span>
            </div>

            <div className="profile-toggle-row">
              <Toggle
                on={controlSounds}
                label="When you mute or deafen"
                onChange={(on) => {
                  setControlSounds(on);
                  setSoundsFor("controls", on);
                  if (on) play("muted");
                }}
              />
              <span className="profile-toggle-label">When you mute, unmute or deafen</span>
            </div>
          </>
        )}
      </div>

      <p className="profile-hint">
        Made rather than recorded, so nothing is downloaded to play them. Rising means
        somebody arrived, falling means somebody left.
      </p>

      <span className="profile-dashed">push to talk and the shortcut · not wired yet</span>
    </>
  );
}

function AlertsTab() {
  return (
    <>
      <span className="profile-title">Alerts</span>
      <p className="profile-hint">
        Nothing notifies anybody of anything today, in the app or from the system. The tab
        stays, empty and honest, rather than offering switches that do nothing.
      </p>
    </>
  );
}

const NAMEPLATES: { id: Nameplate; label: string }[] = [
  { id: "full", label: "Full" },
  { id: "compact", label: "Compact" },
  { id: "none", label: "None" },
];

function LookTab({
  nameplate,
  onNameplate,
}: {
  nameplate: Nameplate;
  onNameplate: (mode: Nameplate) => void;
}) {
  return (
    <>
      <div className="profile-choice-row">
        <button type="button" className="profile-choice" data-active="true" disabled>
          Dark
        </button>
        <button type="button" className="profile-choice" disabled>
          Light
        </button>
      </div>
      <div className="profile-choice-row">
        <button type="button" className="profile-choice" data-active="true" disabled>
          Comfortable
        </button>
        <button type="button" className="profile-choice" disabled>
          Compact
        </button>
      </div>
      <p className="profile-hint">A light theme is a second full palette, not a switch.</p>

      <span className="profile-divider" />

      <span className="profile-title">Whose screen you are watching</span>
      <div className="profile-choice-row">
        {NAMEPLATES.map((mode) => (
          <button
            key={mode.id}
            type="button"
            className="profile-choice"
            data-active={nameplate === mode.id}
            aria-pressed={nameplate === mode.id}
            onClick={() => onNameplate(mode.id)}
          >
            {mode.label}
          </button>
        ))}
      </div>
      <p className="profile-hint">
        Drawn in the corner of somebody else's screen while you watch it. Full carries
        their picture and name, compact only the picture, none stays out of the way.
      </p>

    </>
  );
}

type Verdict = "idle" | "checking" | "current" | "failed";

function AboutTab() {
  const [version, setVersion] = useState<string | null>(null);
  const [verdict, setVerdict] = useState<Verdict>("idle");
  const [release, setRelease] = useState<Release | null>(null);

  useEffect(() => {
    void currentVersion().then(setVersion);
  }, []);

  async function check() {
    setVerdict("checking");
    try {
      const found = await checkForUpdate();
      setRelease(found);
      setVerdict(found ? "idle" : "current");
    } catch {
      setVerdict("failed");
    }
  }

  return (
    <>
      <span className="profile-title">Vocalis</span>
      <div className="profile-card">
        <div className="profile-card-row">
          <span className="profile-field">
            <span className="profile-field-label">Version</span>
            <span className="profile-field-value">{version ?? "in the browser"}</span>
          </span>
          <Button disabled={!version || verdict === "checking"} onClick={() => void check()}>
            {verdict === "checking" ? "Looking…" : "Check for updates"}
          </Button>
        </div>
      </div>
      <p className="profile-hint">
        {verdict === "current" && "You are on the newest version."}
        {verdict === "failed" && "The update server could not be reached."}
        {(verdict === "idle" || verdict === "checking") &&
          (version
            ? "Vocalis also looks for a new version shortly after it starts, and asks before installing one."
            : "Updates only apply to the installed app. The browser always serves the newest build.")}
      </p>

      {release && <UpdateSheet release={release} onClose={() => setRelease(null)} />}
    </>
  );
}
