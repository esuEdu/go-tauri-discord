import { useState } from "react";
import { apiBase } from "../server";

export function fileURL(key: string): string {
  return `${apiBase()}/api/v1/files/${key}`;
}

function initials(name: string): string {
  const trimmed = name.trim();
  if (trimmed === "") return "?";
  return [...trimmed].slice(0, 2).join("").toUpperCase();
}

export function Avatar({
  name,
  imageKey,
  className = "avatar",
}: {
  name: string;
  imageKey?: string | null;
  className?: string;
}) {
  const [broken, setBroken] = useState(false);

  if (!imageKey || broken) {
    return (
      <span className={`${className} lettered`} aria-hidden="true">
        {initials(name)}
      </span>
    );
  }

  return (
    <img
      className={className}
      src={fileURL(imageKey)}
      alt=""
      loading="lazy"
      onError={() => setBroken(true)}
    />
  );
}
