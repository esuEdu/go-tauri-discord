import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { api } from "../api";
import { EmojiPicker } from "./EmojiPicker";
import { Icon } from "./Icon";
import type { Reaction } from "../types/events.gen";

const PICKER_WIDTH = 296;
const EDGE = 12;

function toggle(messageID: string, emoji: string, mine: boolean) {
  const call = mine ? api.unreact(messageID, emoji) : api.react(messageID, emoji);
  void call.catch(() => undefined);
}

type Placement = { left: number; top?: number; bottom?: number; maxHeight: number };

function placeBy(trigger: DOMRect): Placement {
  const above = trigger.top;
  const below = window.innerHeight - trigger.bottom;
  const left = Math.min(
    Math.max(EDGE, trigger.left - PICKER_WIDTH / 2 + trigger.width / 2),
    window.innerWidth - PICKER_WIDTH - EDGE,
  );

  if (above >= below) {
    return { left, bottom: window.innerHeight - trigger.top + 8, maxHeight: above - EDGE - 8 };
  }
  return { left, top: trigger.bottom + 8, maxHeight: below - EDGE - 8 };
}

export function AddReaction({
  messageID,
  reactions,
  className = "reaction add",
  title = "Add a reaction",
}: {
  messageID: string;
  reactions: Reaction[];
  className?: string;
  title?: string;
}) {
  const [picking, setPicking] = useState(false);
  const [where, setWhere] = useState<Placement | null>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!picking || !trigger.current) return;
    setWhere(placeBy(trigger.current.getBoundingClientRect()));
  }, [picking]);

  useEffect(() => {
    if (!picking) return;

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setPicking(false);
    };
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (panel.current?.contains(target) || trigger.current?.contains(target)) return;
      setPicking(false);
    };
    const reflow = () => {
      if (trigger.current) setWhere(placeBy(trigger.current.getBoundingClientRect()));
    };

    window.addEventListener("keydown", onKey);
    window.addEventListener("mousedown", onDown);
    window.addEventListener("resize", reflow);
    window.addEventListener("scroll", reflow, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("resize", reflow);
      window.removeEventListener("scroll", reflow, true);
    };
  }, [picking]);

  return (
    <>
      <button
        ref={trigger}
        className={className}
        title={title}
        aria-expanded={picking}
        onClick={() => setPicking(!picking)}
      >
        <Icon name="smiley-sticker" size={15} />
      </button>

      {picking &&
        where &&
        createPortal(
          <div
            ref={panel}
            className="emoji-picker"
            style={{
              left: where.left,
              top: where.top,
              bottom: where.bottom,
              maxHeight: Math.max(240, where.maxHeight),
            }}
          >
            <EmojiPicker
              onPick={(emoji) => {
                const mine = (reactions ?? []).some((r) => r.emoji === emoji && r.mine);
                toggle(messageID, emoji, mine);
                setPicking(false);
              }}
            />
          </div>,
          document.body,
        )}
    </>
  );
}

export function Reactions({
  messageID,
  reactions,
  mayReact,
}: {
  messageID: string;
  reactions: Reaction[];
  mayReact: boolean;
}) {
  const [who, setWho] = useState<Record<string, string>>({});

  const on = reactions ?? [];
  if (on.length === 0) return null;

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
          onClick={() => toggle(messageID, r.emoji, r.mine)}
        >
          <span>{r.emoji}</span>
          <span className="reaction-count">{r.count}</span>
        </button>
      ))}

      {mayReact && <AddReaction messageID={messageID} reactions={on} />}
    </div>
  );
}
