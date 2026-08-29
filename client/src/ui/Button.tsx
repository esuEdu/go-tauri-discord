import type { ButtonHTMLAttributes } from "react";

export function Button({
  kind = "primary",
  className,
  children,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  kind?: "primary" | "quiet" | "danger";
}) {
  return (
    <button
      type="button"
      className={className ? `button ${className}` : "button"}
      data-kind={kind}
      {...rest}
    >
      {children}
    </button>
  );
}
