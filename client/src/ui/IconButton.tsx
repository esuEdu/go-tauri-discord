import type { ButtonHTMLAttributes } from "react";
import { Icon, type IconName } from "./Icon";

export function IconButton({
  name,
  state = "on",
  size = 15,
  weight = "regular",
  label,
  className,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  name: IconName;
  state?: "on" | "off" | "accent" | "plain" | "danger";
  size?: number;
  weight?: "regular" | "light";
  label: string;
}) {
  return (
    <button
      type="button"
      className={className ? `icon-button ${className}` : "icon-button"}
      data-state={state}
      aria-label={label}
      title={label}
      {...rest}
    >
      <Icon name={name} size={size} weight={weight} />
    </button>
  );
}
