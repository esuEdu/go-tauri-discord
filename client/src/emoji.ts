import { EMOJI_GROUPS, EMOJI_ROWS, type EmojiGroup } from "./emojiList";

export type { EmojiGroup };

export type Emoji = {
  char: string;
  name: string;
  group: EmojiGroup;
  tones: boolean;
  keywords: string;
};

export const TONE_SWATCHES = ["#f7d94c", "#f3d3a0", "#d9a76a", "#a9713f", "#7d5030", "#5a3a22"];

export const EMOJI: Emoji[] = EMOJI_ROWS.map(([char, name, group, tones, keywords]) => ({
  char,
  name,
  group: EMOJI_GROUPS[group],
  tones: tones === 1,
  keywords: keywords ?? "",
}));

const BY_GROUP = new Map<EmojiGroup, Emoji[]>();
for (const emoji of EMOJI) {
  const held = BY_GROUP.get(emoji.group);
  if (held) held.push(emoji);
  else BY_GROUP.set(emoji.group, [emoji]);
}

const BY_CHAR = new Map<string, Emoji>();
for (const emoji of EMOJI) BY_CHAR.set(emoji.char, emoji);

const RECENT_KEY = "emoji_recent";
const MOST_KEY = "emoji_counts";

function read<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

function write(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {}
}

export function recent(): string[] {
  return read<string[]>(RECENT_KEY, []);
}

const STARTERS = ["\u{1F602}", "❤️", "\u{1F62E}", "\u{1F622}", "\u{1F525}", "\u{1F44D}", "\u{1F44E}", "\u{1F389}"];

export function mostUsed(limit = 8): string[] {
  const counts = read<Record<string, number>>(MOST_KEY, {});
  const held = Object.entries(counts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([char]) => char);
  if (held.length >= limit) return held;
  return [...held, ...STARTERS.filter((c) => !held.includes(c))].slice(0, limit);
}

export function remember(char: string) {
  const counts = read<Record<string, number>>(MOST_KEY, {});
  counts[char] = (counts[char] ?? 0) + 1;
  write(MOST_KEY, counts);
  write(RECENT_KEY, [char, ...recent().filter((c) => c !== char)].slice(0, 24));
}

export function nameOf(char: string): string {
  const stripped = char.replace(/[\u{1F3FB}-\u{1F3FF}]/gu, "");
  return (BY_CHAR.get(char) ?? BY_CHAR.get(stripped))?.name ?? "emoji";
}

export function search(term: string, limit = 120): Emoji[] {
  const wanted = term.trim().toLowerCase();
  if (!wanted) return [];
  const words = wanted.split(/\s+/);

  const exact: Emoji[] = [];
  const starts: Emoji[] = [];
  const loose: Emoji[] = [];

  for (const emoji of EMOJI) {
    const name = emoji.name.toLowerCase();
    const hay = `${name} ${emoji.keywords}`;
    if (!words.every((word) => hay.includes(word))) continue;
    if (name === wanted) exact.push(emoji);
    else if (name.startsWith(wanted)) starts.push(emoji);
    else loose.push(emoji);
  }

  return [...exact, ...starts, ...loose].slice(0, limit);
}

export function inGroup(group: EmojiGroup): Emoji[] {
  return BY_GROUP.get(group) ?? [];
}
