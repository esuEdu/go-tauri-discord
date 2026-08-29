import { useEffect, useMemo, useState } from "react";
import { captureSources, type CaptureSource } from "../capture";
import { SCREEN_QUALITIES, type ScreenQualityID } from "../voice";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";

type Tab = "screens" | "apps";

export function GoLive({
  quality,
  onQuality,
  onStart,
  onClose,
}: {
  quality: ScreenQualityID;
  onQuality: (id: ScreenQualityID) => void;
  onStart: () => void;
  onClose: () => void;
}) {
  const [tab, setTab] = useState<Tab>("screens");
  const [sources, setSources] = useState<CaptureSource[] | null>(null);
  const [qualityOpen, setQualityOpen] = useState(false);

  useEffect(() => {
    let dropped = false;
    captureSources().then((found) => {
      if (!dropped) setSources(found);
    });
    return () => {
      dropped = true;
    };
  }, []);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const shown = useMemo(
    () =>
      (sources ?? []).filter(
        (source) => source.kind === (tab === "screens" ? "screen" : "app"),
      ),
    [sources, tab],
  );

  const current = SCREEN_QUALITIES.find((entry) => entry.id === quality);

  return (
    <div className="scrim" onPointerDown={onClose}>
      <div
        className="golive"
        role="dialog"
        aria-label="Go Live"
        onPointerDown={(event) => event.stopPropagation()}
      >
        <span className="golive-title">What do you want to share?</span>

        <div className="tab-switcher">
          <button
            type="button"
            className="tab"
            data-active={tab === "screens"}
            onClick={() => setTab("screens")}
          >
            Screens
          </button>
          <button
            type="button"
            className="tab"
            data-active={tab === "apps"}
            onClick={() => setTab("apps")}
          >
            Apps
          </button>
        </div>

        <div className="golive-grid">
          {sources === null && (
            <span className="golive-note">Looking for something to share…</span>
          )}

          {sources !== null && shown.length === 0 && (
            <span className="golive-note">
              Nothing to show here. Vocalis needs permission to record the screen
              before it can list what you can share.
            </span>
          )}

          {shown.map((source) => (
            <span key={source.id} className="preview-card">
              <span className="preview-picture">
                {source.thumbnail ? (
                  <img src={source.thumbnail} alt="" />
                ) : (
                  <Icon
                    name={
                      source.kind === "screen"
                        ? "monitor-arrow-up"
                        : "app-window"
                    }
                    size={22}
                  />
                )}
              </span>
              <span className="preview-label">{source.title}</span>
            </span>
          ))}
        </div>

        <span className="golive-kicker">Stream quality</span>
        <div className="quality-row">
          <div className="quality-field">
            <button
              type="button"
              className="quality-value"
              aria-expanded={qualityOpen}
              onClick={() => setQualityOpen((was) => !was)}
            >
              <span>{current?.label ?? "Choose"}</span>
              <Icon name="caret-down" size={14} />
            </button>
            {qualityOpen && (
              <div className="quality-options">
                {SCREEN_QUALITIES.map((entry) => (
                  <button
                    key={entry.id}
                    type="button"
                    className="quality-option"
                    data-active={entry.id === quality}
                    onClick={() => {
                      onQuality(entry.id);
                      setQualityOpen(false);
                    }}
                  >
                    <Icon name="speaker-high" size={16} />
                    <span className="quality-option-label">{entry.label}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
          <span className="quality-aside">
            Your system asks which screen when you go live.
          </span>
        </div>

        <div className="golive-actions">
          <Button kind="quiet" onClick={onClose}>
            Cancel
          </Button>
          <Button className="golive-start" onClick={onStart}>
            Go Live…
          </Button>
        </div>
      </div>
    </div>
  );
}
