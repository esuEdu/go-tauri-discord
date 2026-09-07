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

const LOOKING_LIMIT = 12_000;

export async function captureSources(): Promise<CaptureSource[]> {
  if (!onDesktop()) return [];

  const { invoke } = await import("@tauri-apps/api/core");
  const asked = invoke<CaptureSource[]>("capture_sources");

  let timer: ReturnType<typeof setTimeout> | undefined;
  const gaveUp = new Promise<never>((_, fail) => {
    timer = setTimeout(() => fail(new Error("took too long to answer")), LOOKING_LIMIT);
  });

  try {
    return await Promise.race([asked, gaveUp]);
  } finally {
    clearTimeout(timer);
  }
}
