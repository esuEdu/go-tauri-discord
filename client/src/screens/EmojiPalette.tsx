import { useMemo, useState } from "react";
import {
  inGroup,
  mostUsed,
  remember,
  search,
  type Emoji,
  type EmojiGroup,
} from "../emoji";

type Tab = "recent" | EmojiGroup;

const TABS: { id: Tab; glyph: string; label: string }[] = [
  { id: "recent", glyph: "🕒", label: "Frequently used" },
  { id: "smileys", glyph: "🙂", label: "Smileys and people" },
  { id: "animals", glyph: "🐾", label: "Animals and nature" },
  { id: "food", glyph: "🍔", label: "Food and drink" },
  { id: "activity", glyph: "⚽", label: "Activity" },
  { id: "objects", glyph: "💡", label: "Objects" },
  { id: "flags", glyph: "🚩", label: "Flags" },
];

export function EmojiPalette({ onPick }: { onPick: (char: string) => void }) {
  const [tab, setTab] = useState<Tab>("recent");
  const [term, setTerm] = useState("");

  const quick = useMemo(() => mostUsed(24), []);
  const searching = term.trim().length > 0;

  const results = useMemo<Emoji[]>(() => {
    if (!searching) return [];
    return search(term.trim()).filter((emoji) => !quick.includes(emoji.char));
  }, [term, searching, quick]);

  const chars = searching
    ? results.map((emoji) => emoji.char)
    : tab === "recent"
      ? quick
      : inGroup(tab).map((emoji) => emoji.char);

  const heading = searching
    ? `${results.length} ${results.length === 1 ? "result" : "results"} outside your quick picks`
    : (TABS.find((entry) => entry.id === tab)?.label ?? "Frequently used");

  function pick(char: string) {
    remember(char);
    onPick(char);
  }

  return (
    <div className="emoji-palette">
      <input
        className="emoji-search"
        value={term}
        placeholder="Find the perfect emoji"
        autoFocus
        onChange={(event) => setTerm(event.target.value)}
      />

      <div className="emoji-tabs">
        {TABS.map((entry) => (
          <button
            key={entry.id}
            type="button"
            className="emoji-tab"
            data-active={!searching && tab === entry.id}
            aria-label={entry.label}
            title={entry.label}
            onClick={() => {
              setTerm("");
              setTab(entry.id);
            }}
          >
            {entry.glyph}
          </button>
        ))}
      </div>

      <span className="emoji-heading">{heading}</span>

      <div className="emoji-grid">
        {chars.map((char) => (
          <button
            key={char}
            type="button"
            className="emoji-cell"
            onClick={() => pick(char)}
          >
            {char}
          </button>
        ))}
      </div>
    </div>
  );
}
