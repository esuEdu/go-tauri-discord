import { apiBase } from "../server";

export type AvatarSize = 20 | 24 | 28 | 34 | 40 | 56;

export function fileURL(key: string | null | undefined): string | null {
  return key ? `${apiBase()}/api/v1/files/${key}` : null;
}

export function initialsOf(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "?";
  return [...trimmed].slice(0, 2).join("").toUpperCase();
}

export function Avatar({
  name,
  url,
  size = 28,
  tone = "neutral",
  className,
}: {
  name: string;
  url?: string | null;
  size?: AvatarSize;
  tone?: "neutral" | "accent";
  className?: string;
}) {
  const classes = className ? `avatar ${className}` : "avatar";
  return (
    <span className={classes} data-size={size} data-tone={tone} title={name}>
      {url ? <img src={url} alt="" /> : initialsOf(name)}
    </span>
  );
}
