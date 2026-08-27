import { useState } from "react";
import { apiBase } from "../server";

export function fileURL(key: string): string {
  return `${apiBase()}/api/v1/files/${key}`;
}

export function initials(name: string): string {
  const trimmed = name.trim();
  if (trimmed === "") return "?";
  return [...trimmed].slice(0, 2).join("").toUpperCase();
}

export function Avatar({
  name,
  imageKey,
  className,
  mine = false,
}: {
  name: string;
  imageKey?: string | null;
  className?: string;
  mine?: boolean;
}) {
  const [broken, setBroken] = useState(false);
  const classes = ["avatar", mine ? "mine" : null, className].filter(Boolean).join(" ");

  if (!imageKey || broken) {
    return (
      <span className={classes} aria-hidden="true">
        {initials(name)}
      </span>
    );
  }

  return (
    <img
      className={classes}
      src={fileURL(imageKey)}
      alt=""
      loading="lazy"
      onError={() => setBroken(true)}
    />
  );
}
