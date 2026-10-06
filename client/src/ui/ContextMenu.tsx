import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import * as Popover from "@radix-ui/react-popover";
import { useState, type ReactNode } from "react";

export type Anchor = { x: number; y: number };

function anchorStyle(at: Anchor) {
  return {
    position: "fixed" as const,
    left: at.x,
    top: at.y,
    width: 1,
    height: 1,
    pointerEvents: "none" as const,
  };
}

export function ContextMenu({
  at,
  width = 220,
  role = "menu",
  onClose,
  children,
}: {
  at: Anchor;
  width?: number;
  role?: "menu" | "dialog";
  onClose: () => void;
  children: ReactNode;
}) {
  const [opener] = useState<HTMLElement | null>(() =>
    typeof document === "undefined" ? null : (document.activeElement as HTMLElement | null),
  );

  function closed(open: boolean) {
    if (!open) onClose();
  }

  function restore(event: Event) {
    event.preventDefault();
    opener?.focus?.();
  }

  if (role === "dialog") {
    return (
      <Popover.Root open onOpenChange={closed}>
        <Popover.Anchor asChild>
          <span aria-hidden style={anchorStyle(at)} />
        </Popover.Anchor>
        <Popover.Portal>
          <Popover.Content
            className="context-menu"
            style={{ width }}
            side="bottom"
            align="start"
            sideOffset={0}
            collisionPadding={8}
            onCloseAutoFocus={restore}
          >
            {children}
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
    );
  }

  return (
    <DropdownMenu.Root open onOpenChange={closed} modal={false}>
      <DropdownMenu.Trigger asChild>
        <span aria-hidden style={anchorStyle(at)} />
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className="context-menu"
          style={{ width }}
          side="bottom"
          align="start"
          sideOffset={0}
          collisionPadding={8}
          onCloseAutoFocus={restore}
        >
          {children}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
