export type PermissionBit = {
  bit: number;
  name: string;
  about: string;
};

export const PERMISSIONS: PermissionBit[] = [
  { bit: 1 << 0, name: "View channels", about: "See channels and read what is in them" },
  { bit: 1 << 1, name: "Send messages", about: "Post in text channels" },
  { bit: 1 << 2, name: "Manage messages", about: "Delete what other people wrote" },
  { bit: 1 << 3, name: "Manage channels", about: "Create, rename and remove channels" },
  { bit: 1 << 4, name: "Manage roles", about: "Create roles and give them to people" },
  { bit: 1 << 5, name: "Kick members", about: "Remove somebody from the server" },
  { bit: 1 << 6, name: "Ban members", about: "Remove somebody and keep them out" },
  { bit: 1 << 7, name: "Join voice", about: "Enter voice channels" },
  { bit: 1 << 8, name: "Speak", about: "Be heard in voice channels" },
  { bit: 1 << 9, name: "Share a screen", about: "Show a screen in voice channels" },
  { bit: 1 << 10, name: "Administrator", about: "Everything, including anything added later" },
  { bit: 1 << 11, name: "Create invites", about: "Make links that let people join" },
  { bit: 1 << 12, name: "Manage the server", about: "Rename it and change its settings" },
];

export const ADMINISTRATOR = 1 << 10;

export function has(permissions: number, bit: number): boolean {
  return (permissions & bit) === bit;
}

export function withBit(permissions: number, bit: number, on: boolean): number {
  return on ? permissions | bit : permissions & ~bit;
}

export function summarise(permissions: number): string {
  if (has(permissions, ADMINISTRATOR)) return "Everything";
  const held = PERMISSIONS.filter((p) => has(permissions, p.bit));
  if (held.length === 0) return "Nothing";
  if (held.length <= 2) return held.map((p) => p.name).join(", ");
  return `${held.length} permissions`;
}
