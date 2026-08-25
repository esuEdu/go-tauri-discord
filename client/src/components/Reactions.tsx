import { useState } from "react";
import { api } from "../api";
import type { Reaction } from "../types/events.gen";

const PALETTE = ["👍", "👎", "😄", "🎉", "❤️", "👀", "🔥", "✅"];

export function Reactions({
  messageID,
  reactions,
  mayReact,
}: {
  messageID: string;
  reactions: Reaction[];
  mayReact: boolean;
}) {
  const [picking, setPicking] = useState(false);
  const [who, setWho] = useState<Record<string, string>>({});

  const on = reactions ?? [];
  if (on.length === 0 && !mayReact) return null;

  function toggle(emoji: string, mine: boolean) {
    const call = mine ? api.unreact(messageID, emoji) : api.react(messageID, emoji);
    void call.catch(() => undefined);
    setPicking(false);
  }

  async function name(emoji: string, count: number) {
    const at = `${emoji}:${count}`;
    if (who[at] !== undefined) return;
    try {
      const people = await api.reactors(messageID, emoji);
      setWho((prev) => ({ ...prev, [at]: people.map((p) => p.username).join(", ") }));
    } catch {
      setWho((prev) => ({ ...prev, [at]: "" }));
    }
  }

  return (
    <div className="reactions">
      {on.map((r) => (
        <button
          key={r.emoji}
          className={r.mine ? "reaction mine" : "reaction"}
          disabled={!mayReact && !r.mine}
          title={who[`${r.emoji}:${r.count}`] || undefined}
          onMouseEnter={() => void name(r.emoji, r.count)}
          onClick={() => toggle(r.emoji, r.mine)}
        >
          <span className="reaction-emoji">{r.emoji}</span>
          <span className="reaction-count">{r.count}</span>
        </button>
      ))}

      {mayReact && (
        <span className="reaction-picker">
          <button
            className="reaction add"
            title="Add a reaction"
            aria-expanded={picking}
            onClick={() => setPicking(!picking)}
          >
            ☺
          </button>
          {picking && (
            <div className="palette">
              {PALETTE.map((emoji) => (
                <button
                  key={emoji}
                  className="palette-choice"
                  onClick={() => toggle(emoji, on.some((r) => r.emoji === emoji && r.mine))}
                >
                  {emoji}
                </button>
              ))}
            </div>
          )}
        </span>
      )}
    </div>
  );
}
