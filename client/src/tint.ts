const cache = new Map<string, string | null>();
const pending = new Map<string, Promise<string | null>>();

export function tintFor(id: string): string {
  let hash = 0;
  for (const character of id) hash = (hash * 31 + character.codePointAt(0)!) % 360;
  return `hsl(${hash} 22% 45%)`;
}

export function knownTint(url: string): string | null | undefined {
  return cache.get(url);
}

export function tintOf(url: string): Promise<string | null> {
  const held = pending.get(url);
  if (held) return held;

  const work = read(url).then((colour) => {
    cache.set(url, colour);
    pending.delete(url);
    return colour;
  });
  pending.set(url, work);
  return work;
}

function read(url: string): Promise<string | null> {
  return new Promise((resolve) => {
    const picture = new Image();
    picture.crossOrigin = "anonymous";
    picture.onerror = () => resolve(null);
    picture.onload = () => {
      try {
        resolve(dominant(picture));
      } catch {
        resolve(null);
      }
    };
    picture.src = url;
  });
}

const SIDE = 24;

function dominant(picture: HTMLImageElement): string | null {
  const canvas = document.createElement("canvas");
  canvas.width = SIDE;
  canvas.height = SIDE;
  const paper = canvas.getContext("2d", { willReadFrequently: true });
  if (!paper) return null;
  paper.drawImage(picture, 0, 0, SIDE, SIDE);

  const { data } = paper.getImageData(0, 0, SIDE, SIDE);
  const bins = new Map<number, { n: number; r: number; g: number; b: number }>();

  for (let i = 0; i < data.length; i += 4) {
    const [r, g, b, a] = [data[i], data[i + 1], data[i + 2], data[i + 3]];
    if (a < 128) continue;
    const key = ((r >> 4) << 8) | ((g >> 4) << 4) | (b >> 4);
    const bin = bins.get(key);
    if (bin) {
      bin.n += 1;
      bin.r += r;
      bin.g += g;
      bin.b += b;
    } else {
      bins.set(key, { n: 1, r, g, b });
    }
  }

  let best: { n: number; r: number; g: number; b: number } | null = null;
  let bestScore = -1;
  for (const bin of bins.values()) {
    const r = bin.r / bin.n;
    const g = bin.g / bin.n;
    const b = bin.b / bin.n;
    const high = Math.max(r, g, b);
    const low = Math.min(r, g, b);
    const life = high === 0 ? 0 : (high - low) / high;
    const score = bin.n * (0.35 + life);
    if (score > bestScore) {
      bestScore = score;
      best = bin;
    }
  }
  if (!best) return null;
  return calmed(best.r / best.n, best.g / best.n, best.b / best.n);
}

const MAX_SATURATION = 0.36;
const MAX_LIGHTNESS = 0.54;
const MIN_LIGHTNESS = 0.26;

function calmed(r: number, g: number, b: number): string {
  const [red, green, blue] = [r / 255, g / 255, b / 255];
  const high = Math.max(red, green, blue);
  const low = Math.min(red, green, blue);
  const lightness = (high + low) / 2;
  const span = high - low;

  let hue = 0;
  let saturation = 0;
  if (span !== 0) {
    saturation = span / (1 - Math.abs(2 * lightness - 1));
    if (high === red) hue = ((green - blue) / span) % 6;
    else if (high === green) hue = (blue - red) / span + 2;
    else hue = (red - green) / span + 4;
    hue *= 60;
    if (hue < 0) hue += 360;
  }

  const settled = Math.min(saturation, MAX_SATURATION);
  const level = Math.min(Math.max(lightness, MIN_LIGHTNESS), MAX_LIGHTNESS);
  return `hsl(${Math.round(hue)} ${Math.round(settled * 100)}% ${Math.round(level * 100)}%)`;
}
