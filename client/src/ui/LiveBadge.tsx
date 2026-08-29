export function LiveBadge({ tone = "outline" }: { tone?: "outline" | "solid" }) {
  return (
    <span className="badge-live" data-tone={tone}>
      LIVE
    </span>
  );
}
