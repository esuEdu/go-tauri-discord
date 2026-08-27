export type EmojiGroup = "smileys" | "hands" | "animals" | "food" | "flags";

export type Emoji = { char: string; name: string; group: EmojiGroup; tones?: boolean };

export const TONES = ["", "\u{1F3FB}", "\u{1F3FC}", "\u{1F3FD}", "\u{1F3FE}", "\u{1F3FF}"];

export const TONE_SWATCHES = ["#f7d94c", "#f3d3a0", "#d9a76a", "#a9713f", "#7d5030", "#5a3a22"];

export const EMOJI: Emoji[] = [
  { char: "😀", name: "grinning face", group: "smileys" },
  { char: "😃", name: "grinning face with big eyes", group: "smileys" },
  { char: "😄", name: "grinning face with smiling eyes", group: "smileys" },
  { char: "😁", name: "beaming face with smiling eyes", group: "smileys" },
  { char: "😆", name: "grinning squinting face", group: "smileys" },
  { char: "😅", name: "grinning face with sweat", group: "smileys" },
  { char: "🤣", name: "rolling on the floor laughing", group: "smileys" },
  { char: "😂", name: "face with tears of joy", group: "smileys" },
  { char: "🙂", name: "slightly smiling face", group: "smileys" },
  { char: "🙃", name: "upside-down face", group: "smileys" },
  { char: "😉", name: "winking face", group: "smileys" },
  { char: "😊", name: "smiling face with smiling eyes", group: "smileys" },
  { char: "😇", name: "smiling face with halo", group: "smileys" },
  { char: "🥰", name: "smiling face with hearts", group: "smileys" },
  { char: "😍", name: "smiling face with heart-eyes", group: "smileys" },
  { char: "😘", name: "face blowing a kiss", group: "smileys" },
  { char: "😗", name: "kissing face", group: "smileys" },
  { char: "😋", name: "face savouring food", group: "smileys" },
  { char: "😛", name: "face with tongue", group: "smileys" },
  { char: "🤪", name: "zany face", group: "smileys" },
  { char: "🤨", name: "face with raised eyebrow", group: "smileys" },
  { char: "🧐", name: "face with monocle", group: "smileys" },
  { char: "🤓", name: "nerd face", group: "smileys" },
  { char: "😎", name: "smiling face with sunglasses", group: "smileys" },
  { char: "🥳", name: "partying face", group: "smileys" },
  { char: "😏", name: "smirking face", group: "smileys" },
  { char: "😒", name: "unamused face", group: "smileys" },
  { char: "😞", name: "disappointed face", group: "smileys" },
  { char: "😔", name: "pensive face", group: "smileys" },
  { char: "😢", name: "crying face", group: "smileys" },
  { char: "😭", name: "loudly crying face", group: "smileys" },
  { char: "😤", name: "face with steam from nose", group: "smileys" },
  { char: "😠", name: "angry face", group: "smileys" },
  { char: "😡", name: "enraged face", group: "smileys" },
  { char: "🤯", name: "exploding head", group: "smileys" },
  { char: "😳", name: "flushed face", group: "smileys" },
  { char: "🥺", name: "pleading face", group: "smileys" },
  { char: "😱", name: "face screaming in fear", group: "smileys" },
  { char: "😨", name: "fearful face", group: "smileys" },
  { char: "😰", name: "anxious face with sweat", group: "smileys" },
  { char: "😴", name: "sleeping face", group: "smileys" },
  { char: "🤤", name: "drooling face", group: "smileys" },
  { char: "😐", name: "neutral face", group: "smileys" },
  { char: "😑", name: "expressionless face", group: "smileys" },
  { char: "🫡", name: "saluting face", group: "smileys" },
  { char: "🤐", name: "zipper-mouth face", group: "smileys" },
  { char: "🤔", name: "thinking face", group: "smileys" },
  { char: "🤫", name: "shushing face", group: "smileys" },
  { char: "🙄", name: "face with rolling eyes", group: "smileys" },
  { char: "😬", name: "grimacing face", group: "smileys" },
  { char: "🤢", name: "nauseated face", group: "smileys" },
  { char: "🤮", name: "face vomiting", group: "smileys" },
  { char: "🤧", name: "sneezing face", group: "smileys" },
  { char: "🥵", name: "hot face", group: "smileys" },
  { char: "🥶", name: "cold face", group: "smileys" },
  { char: "😵", name: "face with crossed-out eyes", group: "smileys" },
  { char: "🤡", name: "clown face", group: "smileys" },
  { char: "💀", name: "skull", group: "smileys" },
  { char: "👻", name: "ghost", group: "smileys" },
  { char: "👽", name: "alien", group: "smileys" },
  { char: "🤖", name: "robot", group: "smileys" },
  { char: "💩", name: "pile of poo", group: "smileys" },
  { char: "😺", name: "grinning cat", group: "smileys" },
  { char: "😻", name: "smiling cat with heart-eyes", group: "smileys" },
  { char: "🙈", name: "see-no-evil monkey", group: "smileys" },
  { char: "🙉", name: "hear-no-evil monkey", group: "smileys" },
  { char: "🙊", name: "speak-no-evil monkey", group: "smileys" },
  { char: "❤️", name: "red heart", group: "smileys" },
  { char: "🧡", name: "orange heart", group: "smileys" },
  { char: "💛", name: "yellow heart", group: "smileys" },
  { char: "💚", name: "green heart", group: "smileys" },
  { char: "💙", name: "blue heart", group: "smileys" },
  { char: "💜", name: "purple heart", group: "smileys" },
  { char: "🖤", name: "black heart", group: "smileys" },
  { char: "💔", name: "broken heart", group: "smileys" },
  { char: "💯", name: "hundred points", group: "smileys" },
  { char: "🔥", name: "fire", group: "smileys" },
  { char: "✨", name: "sparkles", group: "smileys" },
  { char: "⭐", name: "star", group: "smileys" },
  { char: "🎉", name: "party popper", group: "smileys" },
  { char: "🎊", name: "confetti ball", group: "smileys" },
  { char: "💥", name: "collision", group: "smileys" },
  { char: "💫", name: "dizzy", group: "smileys" },
  { char: "😮", name: "face with open mouth", group: "smileys" },
  { char: "😯", name: "hushed face", group: "smileys" },
  { char: "🥱", name: "yawning face", group: "smileys" },

  { char: "👍", name: "thumbs up", group: "hands", tones: true },
  { char: "👎", name: "thumbs down", group: "hands", tones: true },
  { char: "👌", name: "OK hand", group: "hands", tones: true },
  { char: "🤌", name: "pinched fingers", group: "hands", tones: true },
  { char: "🤏", name: "pinching hand", group: "hands", tones: true },
  { char: "✌️", name: "victory hand", group: "hands", tones: true },
  { char: "🤞", name: "crossed fingers", group: "hands", tones: true },
  { char: "🫰", name: "hand with finger and thumb crossed", group: "hands", tones: true },
  { char: "🤟", name: "love-you gesture", group: "hands", tones: true },
  { char: "🤘", name: "sign of the horns", group: "hands", tones: true },
  { char: "🤙", name: "call me hand", group: "hands", tones: true },
  { char: "👈", name: "backhand index pointing left", group: "hands", tones: true },
  { char: "👉", name: "backhand index pointing right", group: "hands", tones: true },
  { char: "👆", name: "backhand index pointing up", group: "hands", tones: true },
  { char: "👇", name: "backhand index pointing down", group: "hands", tones: true },
  { char: "☝️", name: "index pointing up", group: "hands", tones: true },
  { char: "✋", name: "raised hand", group: "hands", tones: true },
  { char: "🤚", name: "raised back of hand", group: "hands", tones: true },
  { char: "🖐️", name: "hand with fingers splayed", group: "hands", tones: true },
  { char: "🖖", name: "vulcan salute", group: "hands", tones: true },
  { char: "👋", name: "waving hand", group: "hands", tones: true },
  { char: "🤝", name: "handshake", group: "hands" },
  { char: "🙏", name: "folded hands", group: "hands", tones: true },
  { char: "✍️", name: "writing hand", group: "hands", tones: true },
  { char: "💅", name: "nail polish", group: "hands", tones: true },
  { char: "🤳", name: "selfie", group: "hands", tones: true },
  { char: "💪", name: "flexed biceps", group: "hands", tones: true },
  { char: "🦾", name: "mechanical arm", group: "hands" },
  { char: "👏", name: "clapping hands", group: "hands", tones: true },
  { char: "🙌", name: "raising hands", group: "hands", tones: true },
  { char: "👐", name: "open hands", group: "hands", tones: true },
  { char: "🤲", name: "palms up together", group: "hands", tones: true },
  { char: "🫶", name: "heart hands", group: "hands", tones: true },
  { char: "👀", name: "eyes", group: "hands" },
  { char: "👁️", name: "eye", group: "hands" },
  { char: "🧠", name: "brain", group: "hands" },
  { char: "🫀", name: "anatomical heart", group: "hands" },
  { char: "🦴", name: "bone", group: "hands" },
  { char: "👂", name: "ear", group: "hands", tones: true },
  { char: "👃", name: "nose", group: "hands", tones: true },

  { char: "🐶", name: "dog face", group: "animals" },
  { char: "🐱", name: "cat face", group: "animals" },
  { char: "🐭", name: "mouse face", group: "animals" },
  { char: "🐹", name: "hamster", group: "animals" },
  { char: "🐰", name: "rabbit face", group: "animals" },
  { char: "🦊", name: "fox", group: "animals" },
  { char: "🐻", name: "bear", group: "animals" },
  { char: "🐼", name: "panda", group: "animals" },
  { char: "🐨", name: "koala", group: "animals" },
  { char: "🐯", name: "tiger face", group: "animals" },
  { char: "🦁", name: "lion", group: "animals" },
  { char: "🐮", name: "cow face", group: "animals" },
  { char: "🐷", name: "pig face", group: "animals" },
  { char: "🐸", name: "frog", group: "animals" },
  { char: "🐵", name: "monkey face", group: "animals" },
  { char: "🐔", name: "chicken", group: "animals" },
  { char: "🐧", name: "penguin", group: "animals" },
  { char: "🐦", name: "bird", group: "animals" },
  { char: "🦆", name: "duck", group: "animals" },
  { char: "🦉", name: "owl", group: "animals" },
  { char: "🦇", name: "bat", group: "animals" },
  { char: "🐺", name: "wolf", group: "animals" },
  { char: "🐗", name: "boar", group: "animals" },
  { char: "🐴", name: "horse face", group: "animals" },
  { char: "🦄", name: "unicorn", group: "animals" },
  { char: "🐝", name: "honeybee", group: "animals" },
  { char: "🐛", name: "bug", group: "animals" },
  { char: "🦋", name: "butterfly", group: "animals" },
  { char: "🐌", name: "snail", group: "animals" },
  { char: "🐞", name: "lady beetle", group: "animals" },
  { char: "🕷️", name: "spider", group: "animals" },
  { char: "🐢", name: "turtle", group: "animals" },
  { char: "🐍", name: "snake", group: "animals" },
  { char: "🦖", name: "T-Rex", group: "animals" },
  { char: "🐙", name: "octopus", group: "animals" },
  { char: "🦑", name: "squid", group: "animals" },
  { char: "🦐", name: "shrimp", group: "animals" },
  { char: "🐠", name: "tropical fish", group: "animals" },
  { char: "🐬", name: "dolphin", group: "animals" },
  { char: "🐳", name: "spouting whale", group: "animals" },
  { char: "🦈", name: "shark", group: "animals" },
  { char: "🌵", name: "cactus", group: "animals" },
  { char: "🌲", name: "evergreen tree", group: "animals" },
  { char: "🌳", name: "deciduous tree", group: "animals" },
  { char: "🍀", name: "four leaf clover", group: "animals" },
  { char: "🌸", name: "cherry blossom", group: "animals" },
  { char: "🌻", name: "sunflower", group: "animals" },
  { char: "🌹", name: "rose", group: "animals" },
  { char: "🌈", name: "rainbow", group: "animals" },
  { char: "☀️", name: "sun", group: "animals" },
  { char: "🌙", name: "crescent moon", group: "animals" },
  { char: "⚡", name: "high voltage", group: "animals" },
  { char: "❄️", name: "snowflake", group: "animals" },
  { char: "🌊", name: "water wave", group: "animals" },

  { char: "🍏", name: "green apple", group: "food" },
  { char: "🍎", name: "red apple", group: "food" },
  { char: "🍐", name: "pear", group: "food" },
  { char: "🍊", name: "tangerine", group: "food" },
  { char: "🍋", name: "lemon", group: "food" },
  { char: "🍌", name: "banana", group: "food" },
  { char: "🍉", name: "watermelon", group: "food" },
  { char: "🍇", name: "grapes", group: "food" },
  { char: "🍓", name: "strawberry", group: "food" },
  { char: "🫐", name: "blueberries", group: "food" },
  { char: "🍒", name: "cherries", group: "food" },
  { char: "🍑", name: "peach", group: "food" },
  { char: "🥭", name: "mango", group: "food" },
  { char: "🍍", name: "pineapple", group: "food" },
  { char: "🥥", name: "coconut", group: "food" },
  { char: "🥑", name: "avocado", group: "food" },
  { char: "🍅", name: "tomato", group: "food" },
  { char: "🥕", name: "carrot", group: "food" },
  { char: "🌽", name: "ear of corn", group: "food" },
  { char: "🌶️", name: "hot pepper", group: "food" },
  { char: "🥔", name: "potato", group: "food" },
  { char: "🍞", name: "bread", group: "food" },
  { char: "🥐", name: "croissant", group: "food" },
  { char: "🥖", name: "baguette", group: "food" },
  { char: "🧀", name: "cheese wedge", group: "food" },
  { char: "🥚", name: "egg", group: "food" },
  { char: "🍳", name: "cooking", group: "food" },
  { char: "🥞", name: "pancakes", group: "food" },
  { char: "🥓", name: "bacon", group: "food" },
  { char: "🍔", name: "hamburger", group: "food" },
  { char: "🍟", name: "french fries", group: "food" },
  { char: "🍕", name: "pizza", group: "food" },
  { char: "🌭", name: "hot dog", group: "food" },
  { char: "🌮", name: "taco", group: "food" },
  { char: "🌯", name: "burrito", group: "food" },
  { char: "🥗", name: "green salad", group: "food" },
  { char: "🍝", name: "spaghetti", group: "food" },
  { char: "🍜", name: "steaming bowl", group: "food" },
  { char: "🍣", name: "sushi", group: "food" },
  { char: "🍤", name: "fried shrimp", group: "food" },
  { char: "🍦", name: "soft ice cream", group: "food" },
  { char: "🍩", name: "doughnut", group: "food" },
  { char: "🍪", name: "cookie", group: "food" },
  { char: "🎂", name: "birthday cake", group: "food" },
  { char: "🍰", name: "shortcake", group: "food" },
  { char: "🍫", name: "chocolate bar", group: "food" },
  { char: "🍿", name: "popcorn", group: "food" },
  { char: "☕", name: "hot beverage", group: "food" },
  { char: "🍵", name: "teacup", group: "food" },
  { char: "🍺", name: "beer mug", group: "food" },
  { char: "🍻", name: "clinking beer mugs", group: "food" },
  { char: "🥂", name: "clinking glasses", group: "food" },
  { char: "🍷", name: "wine glass", group: "food" },
  { char: "🥃", name: "tumbler glass", group: "food" },
  { char: "🧃", name: "beverage box", group: "food" },
  { char: "🧉", name: "mate", group: "food" },

  { char: "🏳️", name: "white flag", group: "flags" },
  { char: "🏴", name: "black flag", group: "flags" },
  { char: "🏁", name: "chequered flag", group: "flags" },
  { char: "🚩", name: "triangular flag", group: "flags" },
  { char: "🏳️‍🌈", name: "rainbow flag", group: "flags" },
  { char: "🏳️‍⚧️", name: "transgender flag", group: "flags" },
  { char: "🇧🇷", name: "flag Brazil", group: "flags" },
  { char: "🇵🇹", name: "flag Portugal", group: "flags" },
  { char: "🇪🇸", name: "flag Spain", group: "flags" },
  { char: "🇫🇷", name: "flag France", group: "flags" },
  { char: "🇩🇪", name: "flag Germany", group: "flags" },
  { char: "🇮🇹", name: "flag Italy", group: "flags" },
  { char: "🇬🇧", name: "flag United Kingdom", group: "flags" },
  { char: "🇮🇪", name: "flag Ireland", group: "flags" },
  { char: "🇳🇱", name: "flag Netherlands", group: "flags" },
  { char: "🇸🇪", name: "flag Sweden", group: "flags" },
  { char: "🇳🇴", name: "flag Norway", group: "flags" },
  { char: "🇩🇰", name: "flag Denmark", group: "flags" },
  { char: "🇵🇱", name: "flag Poland", group: "flags" },
  { char: "🇺🇦", name: "flag Ukraine", group: "flags" },
  { char: "🇺🇸", name: "flag United States", group: "flags" },
  { char: "🇨🇦", name: "flag Canada", group: "flags" },
  { char: "🇲🇽", name: "flag Mexico", group: "flags" },
  { char: "🇦🇷", name: "flag Argentina", group: "flags" },
  { char: "🇯🇵", name: "flag Japan", group: "flags" },
  { char: "🇰🇷", name: "flag South Korea", group: "flags" },
  { char: "🇨🇳", name: "flag China", group: "flags" },
  { char: "🇮🇳", name: "flag India", group: "flags" },
  { char: "🇦🇺", name: "flag Australia", group: "flags" },
  { char: "🇿🇦", name: "flag South Africa", group: "flags" },
];

const RECENT_KEY = "emoji_recent";
const TONE_KEY = "emoji_tone";
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

export function chosenTone(): number {
  const at = Number(localStorage.getItem(TONE_KEY));
  return Number.isInteger(at) && at >= 0 && at < TONES.length ? at : 0;
}

export function chooseTone(at: number) {
  localStorage.setItem(TONE_KEY, String(at));
}

export function withTone(emoji: Emoji, at: number): string {
  if (!emoji.tones || at === 0) return emoji.char;
  return emoji.char.replace(/️$/, "") + TONES[at];
}

export function recent(): string[] {
  return read<string[]>(RECENT_KEY, []);
}

const STARTERS = ["\u{1F602}", "\u2764\uFE0F", "\u{1F62E}", "\u{1F622}", "\u{1F525}", "\u{1F44D}", "\u{1F44E}", "\u{1F389}"];

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
  const bare = char.replace(/[\u{1F3FB}-\u{1F3FF}]/gu, "");
  return EMOJI.find((e) => e.char === bare || e.char === char)?.name ?? "emoji";
}

export function search(term: string): Emoji[] {
  const wanted = term.trim().toLowerCase();
  if (!wanted) return [];
  const words = wanted.split(/\s+/);
  return EMOJI.filter((e) => words.every((w) => e.name.toLowerCase().includes(w))).slice(0, 64);
}

export function inGroup(group: EmojiGroup): Emoji[] {
  return EMOJI.filter((e) => e.group === group);
}
