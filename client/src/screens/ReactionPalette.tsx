import { useMemo } from "react";
import { mostUsed, remember } from "../emoji";
import { Icon } from "../ui/Icon";

export function ReactionPalette({
  onPick,
  onMore,
}: {
  onPick: (char: string) => void;
  onMore: () => void;
}) {
  const quick = useMemo(() => mostUsed(6), []);

  return (
    <div className="reaction-palette">
      {quick.map((char) => (
        <button
          key={char}
          type="button"
          className="reaction-quick"
          onClick={() => {
            remember(char);
            onPick(char);
          }}
        >
          {char}
        </button>
      ))}
      <span className="reaction-palette-divider" />
      <button
        type="button"
        className="reaction-quick reaction-more"
        aria-label="All emoji"
        title="All emoji"
        onClick={onMore}
      >
        <Icon name="plus" size={16} />
      </button>
    </div>
  );
}
