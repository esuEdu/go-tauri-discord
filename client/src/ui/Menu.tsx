import type { ReactNode } from "react";
import { Icon, type IconName } from "./Icon";

export function Menu({
  children,
  style,
  onPointerDown,
}: {
  children: ReactNode;
  style?: React.CSSProperties;
  onPointerDown?: (event: React.PointerEvent) => void;
}) {
  return (
    <div className="menu" role="menu" style={style} onPointerDown={onPointerDown}>
      {children}
    </div>
  );
}

export function MenuItem({
  icon,
  label,
  hint,
  kind = "normal",
  disabled,
  onClick,
}: {
  icon?: IconName;
  label: string;
  hint?: string;
  kind?: "normal" | "danger";
  disabled?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      className="menu-item"
      role="menuitem"
      data-kind={kind}
      disabled={disabled}
      onClick={onClick}
    >
      {icon && <Icon name={icon} size={16} />}
      <span className="menu-item-label">
        {label}
        {hint && (
          <span className="menu-item-hint" title={hint}>
            {hint}
          </span>
        )}
      </span>
    </button>
  );
}

export function MenuSeparator() {
  return <div className="menu-separator" role="separator" />;
}
