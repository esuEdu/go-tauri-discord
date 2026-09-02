import { onDesktop } from "./capture";

const SKIP_KEY = "update_skipped";

export type Progress = {
  downloaded: number;
  total: number | null;
};

export type Release = {
  version: string;
  notes: string | null;
  install: (onProgress: (progress: Progress) => void) => Promise<void>;
};

function skipped(): string | null {
  try {
    return localStorage.getItem(SKIP_KEY);
  } catch {
    return null;
  }
}

export function skipVersion(version: string) {
  try {
    localStorage.setItem(SKIP_KEY, version);
  } catch {}
}

export async function currentVersion(): Promise<string | null> {
  if (!onDesktop()) return null;
  try {
    const { getVersion } = await import("@tauri-apps/api/app");
    return await getVersion();
  } catch {
    return null;
  }
}

export async function checkForUpdate(): Promise<Release | null> {
  if (!onDesktop()) return null;

  const { check } = await import("@tauri-apps/plugin-updater");
  const found = await check();
  if (!found) return null;

  return {
    version: found.version,
    notes: found.body?.trim() || null,
    install: async (onProgress) => {
      let downloaded = 0;
      let total: number | null = null;

      await found.downloadAndInstall((event) => {
        if (event.event === "Started") {
          total = event.data.contentLength ?? null;
          onProgress({ downloaded, total });
        } else if (event.event === "Progress") {
          downloaded += event.data.chunkLength;
          onProgress({ downloaded, total });
        } else {
          onProgress({ downloaded, total });
        }
      });

      const { relaunch } = await import("@tauri-apps/plugin-process");
      await relaunch();
    },
  };
}

export async function updateOnStartup(): Promise<Release | null> {
  let found: Release | null = null;
  try {
    found = await checkForUpdate();
  } catch {
    return null;
  }
  if (!found || found.version === skipped()) return null;
  return found;
}
