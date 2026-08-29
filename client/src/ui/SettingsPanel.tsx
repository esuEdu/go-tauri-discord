import { useEffect, type ReactNode } from "react";
import { Avatar } from "./Avatar";
import { Icon, type IconName } from "./Icon";

export type SettingsTab<T extends string> = {
  id: T;
  label: string;
  icon: IconName;
};

export function SettingsPanel<T extends string>({
  name,
  kind,
  avatarURL,
  tabs,
  active,
  onPick,
  note,
  onClose,
  children,
}: {
  name: string;
  kind: string;
  avatarURL: string | null;
  tabs: SettingsTab<T>[];
  active: T;
  onPick: (id: T) => void;
  note?: string;
  onClose: () => void;
  children: ReactNode;
}) {
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
        className="settings-panel settings-shell"
        role="dialog"
        aria-label={`${name} ${kind}`}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <nav className="settings-nav">
          <div className="settings-nav-head">
            <Avatar name={name} url={avatarURL} size={28} />
            <span className="settings-nav-text">
              <span className="settings-nav-name">{name}</span>
              <span className="settings-nav-kind">{kind}</span>
            </span>
          </div>
          <div className="settings-tabs">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                className="settings-tab"
                data-active={tab.id === active}
                onClick={() => onPick(tab.id)}
              >
                <Icon name={tab.icon} size={15} />
                {tab.label}
              </button>
            ))}
          </div>
          {note && <p className="settings-nav-note">{note}</p>}
        </nav>

        <div className="settings-content">{children}</div>

        <button
          type="button"
          className="settings-close"
          aria-label="Close"
          title="Close"
          onClick={onClose}
        >
          <Icon name="x" size={15} />
        </button>
      </div>
    </div>
  );
}
