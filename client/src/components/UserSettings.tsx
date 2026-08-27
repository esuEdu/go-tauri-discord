import { useEffect, useState } from "react";
import { api } from "../api";
import {
  chooseMicrophone,
  chosenMicrophone,
  joinsMuted,
  microphones,
  setJoinsMuted,
  type Microphone,
} from "../audioPrefs";
import type { ConnectionState } from "../gateway";
import { serverIsPinned, serverURL } from "../server";
import { Icon } from "./Icon";
import { PickImage } from "./PickImage";
import type { User } from "../types/events.gen";

type Tab = "account" | "voice" | "alerts" | "look";

const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: "account", label: "Account", icon: "user" },
  { id: "voice", label: "Voice", icon: "microphone" },
  { id: "alerts", label: "Alerts", icon: "bell" },
  { id: "look", label: "Look", icon: "paint-brush" },
];

function Connection({ state }: { state: ConnectionState }) {
  const wording: Record<ConnectionState, string> = {
    ready: "Connected",
    connecting: "Connecting",
    reconnecting: "Reconnecting — messages will send when it comes back",
    closed: "Disconnected",
  };

  return (
    <div className="panel-line">
      {state === "closed" ? (
        <Icon name="wifi-slash" size={14} />
      ) : (
        <span className={`presence-dot ${state}`} />
      )}
      <span className="grow clip">{wording[state]}</span>
      {serverIsPinned() && <span className="mono panel-fact">{serverURL()}</span>}
    </div>
  );
}

export function UserSettings({
  user,
  connection,
  onUserChanged,
  onSignOut,
  onDeleteAccount,
}: {
  user: User;
  connection: ConnectionState;
  onUserChanged: (u: User) => void;
  onSignOut: () => void;
  onDeleteAccount: () => void;
}) {
  const [tab, setTab] = useState<Tab>("account");
  const [devices, setDevices] = useState<Microphone[]>([]);
  const [microphone, setMicrophone] = useState(chosenMicrophone() ?? "");
  const [quiet, setQuiet] = useState(joinsMuted());
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (tab !== "voice") return;
    let live = true;
    void microphones().then((found) => live && setDevices(found));
    return () => {
      live = false;
    };
  }, [tab]);

  async function copyTag() {
    try {
      await navigator.clipboard.writeText(`${user.username}#${user.discriminator}`);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="popover" role="dialog" aria-label="Your settings">
      <div className="tabs">
        {TABS.map((t) => (
          <button
            key={t.id}
            className={t.id === tab ? "tab active" : "tab"}
            onClick={() => setTab(t.id)}
          >
            <Icon name={t.icon} size={14} />
            {t.label}
          </button>
        ))}
      </div>

      <div className="popover-body">
        {tab === "account" && (
          <>
            <PickImage
              name={user.username}
              imageKey={user.avatar_key}
              label="a picture"
              className="big"
              mine
              onChosen={(key) => onUserChanged({ ...user, avatar_key: key ?? undefined })}
              upload={async (file, onProgress) => (await api.setAvatar(file, onProgress)).avatar_key}
              remove={() => api.clearAvatar()}
            />

            <div className="panel">
              <div className="panel-line">
                <div className="grow">
                  <div className="panel-label">Name</div>
                  <div className="panel-value">
                    <strong>{user.username}</strong>
                    <span className="mono muted">#{user.discriminator}</span>
                    <button
                      className="link quiet"
                      title="Copy your name and digits"
                      onClick={() => void copyTag()}
                    >
                      <Icon name={copied ? "check" : "copy"} size={13} />
                    </button>
                  </div>
                </div>
                <span className="panel-fact">
                  <Icon name="lock-simple" size={13} />
                  fixed at sign-up
                </span>
              </div>

              <div className="panel-divider" />

              <div className="panel-line">
                <div className="grow">
                  <div className="panel-label">Password</div>
                  <div className="panel-value muted">••••••••</div>
                </div>
                <span className="pending">changing it needs the server</span>
              </div>
            </div>

            <div className="panel">
              <div className="panel-label">Connection</div>
              <Connection state={connection} />
            </div>

            <div className="stack tight">
              <button className="menu-item" onClick={onSignOut}>
                <Icon name="sign-out" size={15} />
                Sign out
              </button>
              <button className="menu-item" onClick={onDeleteAccount}>
                <Icon name="trash" size={15} />
                Delete your account
              </button>
            </div>
          </>
        )}

        {tab === "voice" && (
          <>
            <label className="field">
              Microphone
              <span className="select-wrap">
                <select
                  className="input"
                  value={microphone}
                  onChange={(e) => {
                    setMicrophone(e.target.value);
                    chooseMicrophone(e.target.value === "" ? null : e.target.value);
                  }}
                >
                  <option value="">whatever the browser picked</option>
                  {devices.map((device) => (
                    <option key={device.id} value={device.id}>
                      {device.label}
                    </option>
                  ))}
                </select>
                <Icon name="caret-down" size={13} />
              </span>
              {devices.length === 0 && (
                <span className="field-note">
                  Microphones are only named once you have joined a call at least once.
                </span>
              )}
            </label>

            <button
              className="switch"
              role="switch"
              aria-checked={quiet}
              onClick={() => {
                setQuiet(!quiet);
                setJoinsMuted(!quiet);
              }}
            >
              <span className="switch-track" />
              Join calls with the microphone off
            </button>
            <span className="field-note">A change applies the next time you join.</span>

            <div className="dashed">
              <div className="row">
                <Icon name="sliders-horizontal" size={16} />
                <span className="gate-aside-title">Levels and push to talk</span>
              </div>
              <div className="note">
                Choosing an output device, a level meter before joining, and push to talk all need
                work the server and the voice client do not do yet.
              </div>
            </div>
          </>
        )}

        {tab === "alerts" && (
          <div className="dashed">
            <div className="row">
              <Icon name="bell" size={16} />
              <span className="gate-aside-title">Alerts</span>
            </div>
            <div className="note">
              Nothing notifies anybody of anything today, in the app or from the system. The tab
              stays, empty and honest, rather than offering switches that do nothing.
            </div>
          </div>
        )}

        {tab === "look" && (
          <div className="dashed">
            <div className="row">
              <Icon name="paint-brush" size={16} />
              <span className="gate-aside-title">One theme, for now</span>
            </div>
            <div className="note">
              A light theme is a second full palette, not a switch — elevation, the accent and every
              call state were tuned against a near-black ground. Until that palette exists there is
              nothing here to choose.
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
