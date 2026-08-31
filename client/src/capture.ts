export type CaptureSource = {
  id: string;
  kind: "screen" | "app";
  title: string;
  thumbnail: string | null;
  pid: number | null;
};

export function onDesktop(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

export async function captureSources(): Promise<CaptureSource[]> {
  if (!onDesktop()) return [];
  try {
    const { invoke } = await import("@tauri-apps/api/core");
    return await invoke<CaptureSource[]>("capture_sources");
  } catch {
    return [];
  }
}
