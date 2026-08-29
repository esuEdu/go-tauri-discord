const DRAWN = new Set(["headphones-slash", "monitor-x"]);

export type IconName =
  | "app-window"
  | "arrow-bend-up-left"
  | "arrow-down"
  | "arrow-left"
  | "arrow-up"
  | "caret-down"
  | "check"
  | "copy"
  | "dots-three"
  | "download"
  | "eye"
  | "eye-slash"
  | "folder-plus"
  | "gear-six"
  | "hash"
  | "headphones"
  | "headphones-slash"
  | "microphone"
  | "microphone-slash"
  | "monitor-arrow-up"
  | "monitor-x"
  | "paper-plane-tilt"
  | "paperclip"
  | "pencil-simple"
  | "phone-x"
  | "play"
  | "plus"
  | "sign-out"
  | "smiley"
  | "speaker-high"
  | "speaker-slash"
  | "trash"
  | "user-circle"
  | "wifi-high"
  | "x";

export function Icon({
  name,
  size = 16,
  tone,
  weight = "regular",
  className,
}: {
  name: IconName;
  size?: number;
  tone?: "bad";
  weight?: "regular" | "light";
  className?: string;
}) {
  const colour = tone === "bad" ? "var(--bad-edge)" : undefined;
  if (DRAWN.has(name)) {
    return (
      <span
        className={className ? `icon-drawn ${className}` : "icon-drawn"}
        data-icon={name}
        style={{ width: size, height: size, color: colour }}
        aria-hidden="true"
      />
    );
  }

  const classes = [weight === "light" ? "ph-light" : "ph", `ph-${name}`];
  if (className) classes.push(className);
  return (
    <i
      className={classes.join(" ")}
      style={{ fontSize: size, color: colour }}
      aria-hidden="true"
    />
  );
}
