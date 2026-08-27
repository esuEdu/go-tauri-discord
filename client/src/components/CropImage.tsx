import { useCallback, useEffect, useRef, useState } from "react";
import { Icon } from "./Icon";

const SQUARE = 150;
const OUT = 256;
const MAX_ZOOM = 4;

type Placed = { zoom: number; x: number; y: number };

function coverScale(width: number, height: number): number {
  return SQUARE / Math.min(width, height);
}

function clamp(at: Placed, width: number, height: number): Placed {
  const scale = coverScale(width, height) * at.zoom;
  const drawnW = width * scale;
  const drawnH = height * scale;
  return {
    zoom: at.zoom,
    x: Math.min(0, Math.max(SQUARE - drawnW, at.x)),
    y: Math.min(0, Math.max(SQUARE - drawnH, at.y)),
  };
}

function centred(width: number, height: number, zoom: number): Placed {
  const scale = coverScale(width, height) * zoom;
  return {
    zoom,
    x: (SQUARE - width * scale) / 2,
    y: (SQUARE - height * scale) / 2,
  };
}

function frame(at: Placed, image: HTMLImageElement, size: number) {
  const ratio = size / SQUARE;
  const scale = coverScale(image.naturalWidth, image.naturalHeight) * at.zoom;
  return {
    backgroundImage: `url(${image.src})`,
    backgroundRepeat: "no-repeat",
    backgroundSize: `${image.naturalWidth * scale * ratio}px ${image.naturalHeight * scale * ratio}px`,
    backgroundPosition: `${at.x * ratio}px ${at.y * ratio}px`,
  };
}

export function CropImage({
  file,
  title,
  onCancel,
  onDone,
}: {
  file: File;
  title: string;
  onCancel: () => void;
  onDone: (cropped: File) => void;
}) {
  const [image, setImage] = useState<HTMLImageElement | null>(null);
  const [at, setAt] = useState<Placed>({ zoom: 1, x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [busy, setBusy] = useState(false);
  const from = useRef({ x: 0, y: 0, at: { zoom: 1, x: 0, y: 0 } as Placed });

  useEffect(() => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      setImage(img);
      setAt(centred(img.naturalWidth, img.naturalHeight, 1));
    };
    img.src = url;
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onCancel();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [busy, onCancel]);

  const move = useCallback(
    (e: PointerEvent) => {
      if (!image) return;
      setAt((prev) =>
        clamp(
          {
            zoom: prev.zoom,
            x: from.current.at.x + (e.clientX - from.current.x),
            y: from.current.at.y + (e.clientY - from.current.y),
          },
          image.naturalWidth,
          image.naturalHeight,
        ),
      );
    },
    [image],
  );

  useEffect(() => {
    if (!dragging) return;
    const stop = () => setDragging(false);
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
    return () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
    };
  }, [dragging, move]);

  function rezoom(zoom: number) {
    if (!image) return;
    setAt((prev) => {
      const before = coverScale(image.naturalWidth, image.naturalHeight) * prev.zoom;
      const after = coverScale(image.naturalWidth, image.naturalHeight) * zoom;
      const middle = SQUARE / 2;
      return clamp(
        {
          zoom,
          x: middle - ((middle - prev.x) / before) * after,
          y: middle - ((middle - prev.y) / before) * after,
        },
        image.naturalWidth,
        image.naturalHeight,
      );
    });
  }

  async function use() {
    if (!image || busy) return;
    setBusy(true);
    const scale = coverScale(image.naturalWidth, image.naturalHeight) * at.zoom;
    const canvas = document.createElement("canvas");
    canvas.width = OUT;
    canvas.height = OUT;
    const pen = canvas.getContext("2d");
    if (!pen) {
      setBusy(false);
      return;
    }
    pen.imageSmoothingQuality = "high";
    pen.drawImage(
      image,
      -at.x / scale,
      -at.y / scale,
      SQUARE / scale,
      SQUARE / scale,
      0,
      0,
      OUT,
      OUT,
    );

    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, "image/png"),
    );
    if (!blob) {
      setBusy(false);
      return;
    }
    onDone(new File([blob], file.name.replace(/\.[^.]+$/, "") + ".png", { type: "image/png" }));
  }

  const tooSmall = image ? Math.min(image.naturalWidth, image.naturalHeight) < OUT : false;

  return (
    <div className="dialog-backdrop" onMouseDown={() => !busy && onCancel()}>
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div>
          <div className="dialog-title">{title}</div>
          <div className="dialog-sub">what is inside the square is what everyone sees</div>
        </div>

        <div className="crop-stage">
          {image && (
            <>
              <div className="crop-blur" style={frame(at, image, SQUARE)} />
              <div
                className="crop-window"
                style={frame(at, image, SQUARE)}
                onPointerDown={(e) => {
                  from.current = { x: e.clientX, y: e.clientY, at };
                  setDragging(true);
                }}
              />
              <span className="crop-hint">
                <Icon name="arrows-out-cardinal" size={12} />
                drag to move
              </span>
            </>
          )}
        </div>

        <div className="crop-zoom">
          <Icon name="magnifying-glass-minus" size={16} />
          <input
            type="range"
            min={100}
            max={MAX_ZOOM * 100}
            step={5}
            value={Math.round(at.zoom * 100)}
            aria-label="Zoom"
            onChange={(e) => rezoom(Number(e.target.value) / 100)}
          />
          <Icon name="magnifying-glass-plus" size={16} />
          <span className="crop-factor">{at.zoom.toFixed(1)}×</span>
        </div>

        {image && (
          <div className="crop-previews">
            <span className="crop-preview" style={{ ...frame(at, image, 44), width: 44, height: 44 }} />
            <span className="crop-preview" style={{ ...frame(at, image, 26), width: 26, height: 26 }} />
            <span className="crop-preview" style={{ ...frame(at, image, 20), width: 20, height: 20 }} />
            <span className="field-note">
              how it lands beside your name, in a call, and in the roster
            </span>
          </div>
        )}

        {tooSmall && image && (
          <div className="banner bad">
            <Icon name="warning-circle" size={15} />
            <span>
              That picture is {Math.min(image.naturalWidth, image.naturalHeight)} px across, so the
              square cannot fill {OUT} without going soft. You can use it as it is, or pick a bigger
              one.
            </span>
          </div>
        )}

        <div className="row">
          <button
            className="btn btn-quiet btn-small"
            disabled={!image}
            onClick={() => image && setAt(centred(image.naturalWidth, image.naturalHeight, 1))}
          >
            <Icon name="arrow-counter-clockwise" size={14} />
            Reset
          </button>
          <span className="grow" />
          <button className="btn btn-quiet" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button className="btn btn-primary" onClick={() => void use()} disabled={!image || busy}>
            {busy ? "…" : "Use this"}
          </button>
        </div>
      </div>
    </div>
  );
}
