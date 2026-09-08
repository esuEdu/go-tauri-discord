import { onDesktop } from "./capture";
import { offerInvite } from "./invites";

export async function watchInviteLinks(): Promise<void> {
  if (!onDesktop()) return;

  try {
    const { getCurrent, onOpenUrl } = await import("@tauri-apps/plugin-deep-link");
    for (const url of (await getCurrent()) ?? []) offerInvite(url);
    await onOpenUrl((urls) => {
      for (const url of urls) offerInvite(url);
    });
  } catch (cause) {
    console.warn("invite links will not open this window", cause);
  }
}
