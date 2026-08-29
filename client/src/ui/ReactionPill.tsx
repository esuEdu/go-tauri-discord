export function ReactionPill({
  emoji,
  count,
  mine,
  title,
  onClick,
  onPointerEnter,
}: {
  emoji: string;
  count: number;
  mine: boolean;
  title?: string;
  onClick?: () => void;
  onPointerEnter?: () => void;
}) {
  return (
    <button
      type="button"
      className="reaction-pill"
      data-mine={mine ? "yes" : "no"}
      title={title}
      onClick={onClick}
      onPointerEnter={onPointerEnter}
    >
      <span>{emoji}</span>
      <span className="reaction-pill-count">{count}</span>
    </button>
  );
}
