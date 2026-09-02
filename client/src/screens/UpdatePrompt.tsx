import { useEffect, useState } from "react";
import { skipVersion, updateOnStartup, type Progress, type Release } from "../updates";
import { Button } from "../ui/Button";
import { Sheet } from "../ui/Sheet";

const STARTUP_DELAY = 4000;

const stay = () => {};

function megabytes(bytes: number): string {
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export function UpdateSheet({
  release,
  onClose,
}: {
  release: Release;
  onClose: () => void;
}) {
  const [progress, setProgress] = useState<Progress | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const installing = progress !== null && failure === null;

  async function install() {
    setFailure(null);
    setProgress({ downloaded: 0, total: null });
    try {
      await release.install(setProgress);
    } catch (cause) {
      setProgress(null);
      setFailure(cause instanceof Error ? cause.message : String(cause));
    }
  }

  const share =
    progress && progress.total ? Math.min(1, progress.downloaded / progress.total) : null;

  return (
    <Sheet
      title={`Vocalis ${release.version} is out`}
      subtitle="Downloading it replaces the app in place. Vocalis restarts once it lands, so leave any call you are in first."
      width={420}
      onClose={installing ? stay : onClose}
    >
      {release.notes && <p className="update-notes">{release.notes}</p>}

      {installing && (
        <div className="update-progress">
          <div className="update-track">
            <span
              className="update-fill"
              data-unknown={share === null}
              style={share === null ? undefined : { width: `${share * 100}%` }}
            />
          </div>
          <span className="update-progress-text">
            {progress.total
              ? `${megabytes(progress.downloaded)} of ${megabytes(progress.total)}`
              : "Downloading…"}
          </span>
        </div>
      )}

      {failure && <p className="update-failure">The update did not install: {failure}</p>}

      {!installing && (
        <div className="sheet-actions">
          <Button
            kind="quiet"
            onClick={() => {
              skipVersion(release.version);
              onClose();
            }}
          >
            Skip this one
          </Button>
          <Button onClick={() => void install()}>
            {failure ? "Try again" : "Update and restart"}
          </Button>
        </div>
      )}
    </Sheet>
  );
}

export function StartupUpdate() {
  const [release, setRelease] = useState<Release | null>(null);

  useEffect(() => {
    let cancelled = false;
    const timer = setTimeout(() => {
      void updateOnStartup().then((found) => {
        if (!cancelled) setRelease(found);
      });
    }, STARTUP_DELAY);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, []);

  if (!release) return null;
  return <UpdateSheet release={release} onClose={() => setRelease(null)} />;
}
