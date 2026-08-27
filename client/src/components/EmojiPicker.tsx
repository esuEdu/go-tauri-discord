import { useMemo, useRef, useState } from "react";
import {
  EMOJI,
  TONE_SWATCHES,
  chooseTone,
  chosenTone,
  inGroup,
  mostUsed,
  nameOf,
  recent,
  remember,
  search,
  withTone,
  type Emoji,
  type EmojiGroup,
} from "../emoji";
import { Icon } from "./Icon";

type Tab = "recent" | EmojiGroup;

const TABS: { id: Tab; icon: string; label: string }[] = [
  { id: "recent", icon: "clock-counter-clockwise", label: "Recent" },
  { id: "smileys", icon: "smiley", label: "Smileys" },
  { id: "hands", icon: "hand", label: "Hands" },
  { id: "animals", icon: "cat", label: "Animals and nature" },
  { id: "food", icon: "hamburger", label: "Food and drink" },
  { id: "flags", icon: "flag", label: "Flags" },
];

const TAB_TITLES: Record<Tab, string> = {
  recent: "Lately",
  smileys: "Smileys",
  hands: "Hands",
  animals: "Animals and nature",
  food: "Food and drink",
  flags: "Flags",
};

function byChar(chars: string[]): Emoji[] {
  return chars
    .map((c) => EMOJI.find((e) => e.char === c.replace(/[\u{1F3FB}-\u{1F3FF}]/gu, "")))
    .filter((e): e is Emoji => Boolean(e));
}

export function EmojiPicker({ onPick }: { onPick: (emoji: string) => void }) {
  const [tab, setTab] = useState<Tab>("recent");
  const [term, setTerm] = useState("");
  const [tone, setTone] = useState(chosenTone);
  const [hovered, setHovered] = useState<Emoji | null>(null);
  const field = useRef<HTMLInputElement>(null);

  const found = useMemo(() => search(term), [term]);
  const most = useMemo(() => byChar(mostUsed()), []);
  const lately = useMemo(() => byChar(recent()), []);

  function pick(emoji: Emoji) {
    const char = withTone(emoji, tone);
    remember(char);
    onPick(char);
  }

  function grid(title: string, list: Emoji[], empty?: string) {
    if (list.length === 0) {
      return empty ? (
        <>
          <div className="emoji-heading">{title}</div>
          <span className="field-note">{empty}</span>
        </>
      ) : null;
    }
    return (
      <>
        <div className="emoji-heading">{title}</div>
        <div className="emoji-grid">
          {list.map((emoji, at) => (
            <button
              key={`${emoji.char}-${at}`}
              className="emoji-cell"
              title={emoji.name}
              onMouseEnter={() => setHovered(emoji)}
              onFocus={() => setHovered(emoji)}
              onClick={() => pick(emoji)}
            >
              {withTone(emoji, tone)}
            </button>
          ))}
        </div>
      </>
    );
  }

  const showing = hovered ?? found[0] ?? lately[0] ?? most[0] ?? EMOJI[0];

  return (
    <div className="emoji-picker">
      <div className="emoji-search">
        <Icon name="magnifying-glass" size={14} />
        <input
          ref={field}
          className="emoji-search-input"
          placeholder="Search every emoji"
          value={term}
          autoFocus
          onChange={(e) => setTerm(e.target.value)}
        />
        {term && (
          <button className="composer-icon" aria-label="Clear the search" onClick={() => setTerm("")}>
            <Icon name="x" size={13} />
          </button>
        )}
      </div>

      {!term && (
        <div className="emoji-tabs">
          {TABS.map((t) => (
            <button
              key={t.id}
              className={t.id === tab ? "emoji-tab active" : "emoji-tab"}
              title={t.label}
              aria-label={t.label}
              onClick={() => setTab(t.id)}
            >
              <Icon name={t.icon} size={14} />
            </button>
          ))}
        </div>
      )}

      <div className="emoji-body">
        {term ? (
          grid(
            found.length > 0 ? `${found.length} match${found.length === 1 ? "" : "es"}` : "Nothing",
            found,
            "No emoji goes by that name.",
          )
        ) : tab === "recent" ? (
          <>
            {grid("Used most, by you", most, "Nothing yet — pick one and it lands here.")}
            {grid("Lately", lately)}
          </>
        ) : (
          grid(TAB_TITLES[tab], inGroup(tab))
        )}
      </div>

      <div className="emoji-foot">
        <span className="emoji-foot-char">{withTone(showing, tone)}</span>
        <span className="emoji-foot-name">{nameOf(showing.char)}</span>
        <span className="emoji-tones">
          {TONE_SWATCHES.map((colour, at) => (
            <button
              key={colour}
              className={at === tone ? "emoji-tone picked" : "emoji-tone"}
              style={{ background: colour }}
              aria-label={at === 0 ? "Default skin tone" : `Skin tone ${at}`}
              onClick={() => {
                setTone(at);
                chooseTone(at);
              }}
            />
          ))}
        </span>
      </div>
    </div>
  );
}
