import { useMemo, useState } from "react";
import {
  inGroup,
  mostUsed,
  remember,
  search,
  type Emoji,
  type EmojiGroup,
} from "../emoji";

const TABS: { id: EmojiGroup; glyph: string; label: string }[] = [
  { id: "smileys", glyph: "🙂", label: "Smileys and people" },
  { id: "animals", glyph: "🐾", label: "Animals and nature" },
  { id: "food", glyph: "🍔", label: "Food and drink" },
  { id: "activity", glyph: "⚽", label: "Activity and travel" },
  { id: "objects", glyph: "💡", label: "Objects and symbols" },
  { id: "flags", glyph: "🚩", label: "Flags" },
];

export function EmojiPalette({ onPick }: { onPick: (char: string) => void }) {
  const [tab, setTab] = useState<EmojiGroup>("smileys");
  const [term, setTerm] = useState("");

  const quick = useMemo(() => mostUsed(8), []);
  const searching = term.trim().length > 0;

  const results = useMemo<Emoji[]>(
    () => (searching ? search(term.trim()) : []),
    [term, searching],
  );

  const chars = searching
    ? results.map((emoji) => emoji.char)
    : inGroup(tab).map((emoji) => emoji.char);

  const heading = searching
    ? `${results.length} ${results.length === 1 ? "result" : "results"}`
    : (TABS.find((entry) => entry.id === tab)?.label ?? "");

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

      {!searching && quick.length > 0 && (
        <>
          <span className="emoji-heading">Used most, by you</span>
          <div className="emoji-quick">
            {quick.map((char) => (
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
        </>
      )}

      <span className="emoji-heading">{heading}</span>

      <div className="emoji-grid">
        {chars.length === 0 ? (
          <span className="emoji-empty">Nothing matches that.</span>
        ) : (
          chars.map((char) => (
            <button
              key={char}
              type="button"
              className="emoji-cell"
              onClick={() => pick(char)}
            >
              {char}
            </button>
          ))
        )}
      </div>
    </div>
  );
}
