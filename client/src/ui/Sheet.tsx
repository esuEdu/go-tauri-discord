import * as Dialog from "@radix-ui/react-dialog";
import { useState, type ReactNode } from "react";

export function Sheet({
  title,
  subtitle,
  width,
  className,
  onClose,
  children,
}: {
  title: string;
  subtitle?: string;
  width?: number;
  className?: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const described = subtitle ? {} : { "aria-describedby": undefined };
  const [opener] = useState<HTMLElement | null>(() =>
    typeof document === "undefined" ? null : (document.activeElement as HTMLElement | null),
  );

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="sheet-scrim" />
        <Dialog.Content
          className={className ? `sheet ${className}` : "sheet"}
          style={width ? { width } : undefined}
          aria-modal="true"
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            opener?.focus?.();
          }}
          {...described}
        >
          {subtitle ? (
            <div className="sheet-head">
              <Dialog.Title asChild>
                <span className="sheet-title">{title}</span>
              </Dialog.Title>
              <Dialog.Description asChild>
                <span className="sheet-subtitle">{subtitle}</span>
              </Dialog.Description>
            </div>
          ) : (
            <Dialog.Title asChild>
              <span className="sheet-title">{title}</span>
            </Dialog.Title>
          )}
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
