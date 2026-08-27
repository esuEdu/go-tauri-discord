export function Icon({
  name,
  size = 16,
  className,
}: {
  name: string;
  size?: number;
  className?: string;
}) {
  return (
    <i
      className={className ? `ph ph-${name} ${className}` : `ph ph-${name}`}
      style={{ fontSize: size }}
      aria-hidden="true"
    />
  );
}
