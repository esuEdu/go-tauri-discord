import { useEffect, useState } from "react";
import { api } from "../api";
import {
  chosenMicrophone,
  joinsMuted,
  microphones,
  setJoinsMuted,
  type Microphone,
} from "../audioPrefs";
import type { User } from "../types/events.gen";
import { checkForUpdate, currentVersion, type Release } from "../updates";
import { Avatar } from "../ui/Avatar";
import { Sheet } from "../ui/Sheet";
import { PlacePicture } from "./PlacePicture";
import { UpdateSheet } from "./UpdatePrompt";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Toggle } from "../ui/Toggle";
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
  avatarURL,
  onClose,
  onChanged,
  onSignOut,
  onDeleteAccount,
}: {
  user: User;
  avatarURL: string | null;
  onClose: () => void;
  onChanged: () => void;
  onSignOut: () => void;
  onDeleteAccount: () => void;
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
              avatarURL={avatarURL}
              onChanged={onChanged}
              onSignOut={onSignOut}
              onDeleteAccount={onDeleteAccount}
            />
          )}
          {tab === "voice" && <VoiceTab />}
          {tab === "alerts" && <AlertsTab />}
          {tab === "look" && <LookTab />}
          {tab === "about" && <AboutTab />}
        </div>
      </div>
    </div>
  );
}

function AccountTab({
  user,
  avatarURL,
  onChanged,
  onSignOut,
  onDeleteAccount,
}: {
  user: User;
  avatarURL: string | null;
  onChanged: () => void;
  onSignOut: () => void;
  onDeleteAccount: () => void;
}) {
  const [placing, setPlacing] = useState<File | null>(null);
  const [removing, setRemoving] = useState(false);

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

  useEffect(() => {
    void microphones().then(setMics);
  }, []);

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
        <span className="profile-select" aria-disabled="true">
          system default
        </span>
      </div>

      <div className="profile-level">
        <div className="profile-level-head">
          <span>Your level, as others hear it</span>
          <span>100%</span>
        </div>
        <div className="profile-level-track">
          <span className="profile-level-knob" />
        </div>
        <div className="profile-meter">
          {Array.from({ length: 10 }, (_, i) => (
            <span key={i} className="profile-meter-bar" data-lit={i < 4} />
          ))}
        </div>
        <p className="profile-hint">
          The meter is the only way to tell a dead microphone from a quiet room before
          joining.
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

      <span className="profile-dashed">every control here · needs backend</span>
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

function LookTab() {
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
