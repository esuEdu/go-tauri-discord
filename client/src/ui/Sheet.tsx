import { useEffect, type ReactNode } from "react";

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
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="scrim" onPointerDown={onClose}>
      <div
        className={className ? `sheet ${className}` : "sheet"}
        role="dialog"
        aria-label={title}
        style={width ? { width } : undefined}
        onPointerDown={(event) => event.stopPropagation()}
      >
        {subtitle ? (
          <div className="sheet-head">
            <span className="sheet-title">{title}</span>
            <span className="sheet-subtitle">{subtitle}</span>
          </div>
        ) : (
          <span className="sheet-title">{title}</span>
        )}
        {children}
      </div>
    </div>
  );
}
