import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";

export type Anchor = { x: number; y: number };

export function ContextMenu({
  at,
  width = 220,
  onClose,
  children,
}: {
  at: Anchor;
  width?: number;
  onClose: () => void;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [place, setPlace] = useState<{ left: number; top: number }>({
    left: at.x,
    top: at.y,
  });

  useLayoutEffect(() => {
    const node = ref.current;
    if (!node) return;
    const margin = 8;
    const box = node.getBoundingClientRect();
    const left = Math.max(
      margin,
      Math.min(at.x, window.innerWidth - box.width - margin),
    );
    const top = Math.max(
      margin,
      Math.min(at.y, window.innerHeight - box.height - margin),
    );
    setPlace({ left, top });
  }, [at.x, at.y]);

  useEffect(() => {
    function away(event: PointerEvent) {
      if (!ref.current?.contains(event.target as Node)) onClose();
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("pointerdown", away);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("pointerdown", away);
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="context-menu"
      role="menu"
      style={{ left: place.left, top: place.top, width }}
    >
      {children}
    </div>
  );
}
