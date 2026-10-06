import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import type { CSSProperties, ReactNode } from "react";
import { Icon, type IconName } from "./Icon";

export function Menu({
  children,
  style,
  align = "end",
  sideOffset = 6,
}: {
  children: ReactNode;
  style?: CSSProperties;
  align?: "start" | "center" | "end";
  sideOffset?: number;
}) {
  return (
    <DropdownMenu.Portal>
      <DropdownMenu.Content
        className="menu"
        style={style}
        align={align}
        sideOffset={sideOffset}
        collisionPadding={8}
      >
        {children}
      </DropdownMenu.Content>
    </DropdownMenu.Portal>
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
    <DropdownMenu.Item asChild disabled={disabled} onSelect={() => onClick?.()}>
      <button type="button" className="menu-item" data-kind={kind} disabled={disabled}>
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
    </DropdownMenu.Item>
  );
}

export function MenuSeparator() {
  return <DropdownMenu.Separator className="menu-separator" />;
}
