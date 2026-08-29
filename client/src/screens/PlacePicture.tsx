import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "../ui/Button";

const STAGE_WIDTH = 348;
const STAGE_HEIGHT = 210;
const SQUARE = 150;
const OUTPUT = 256;
const MAX_ZOOM = 3;

type Placement = { zoom: number; x: number; y: number };

const CENTRED: Placement = { zoom: 1, x: 0, y: 0 };

export function PlacePicture({
  file,
  onCancel,
  onUse,
}: {
  file: File;
  onCancel: () => void;
  onUse: (cropped: File) => void;
}) {
  const [source, setSource] = useState<HTMLImageElement | null>(null);
  const [place, setPlace] = useState<Placement>(CENTRED);
  const [busy, setBusy] = useState(false);
  const drag = useRef<{ x: number; y: number; from: Placement } | null>(null);

  useEffect(() => {
    const url = URL.createObjectURL(file);
    const picture = new Image();
    picture.onload = () => setSource(picture);
    picture.src = url;
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onCancel();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onCancel]);

  const held = useCallback(
    (next: Placement): Placement => {
      if (!source) return next;
      const covered = cover(source, next.zoom);
      const room = {
        x: Math.max(0, (covered.width - SQUARE) / 2),
        y: Math.max(0, (covered.height - SQUARE) / 2),
      };
      return {
        zoom: next.zoom,
        x: Math.min(Math.max(next.x, -room.x), room.x),
        y: Math.min(Math.max(next.y, -room.y), room.y),
      };
    },
    [source],
  );

  useEffect(() => {
    setPlace((was) => held(was));
  }, [held]);

  function onPointerDown(event: React.PointerEvent<HTMLDivElement>) {
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = { x: event.clientX, y: event.clientY, from: place };
  }

  function onPointerMove(event: React.PointerEvent<HTMLDivElement>) {
    const held0 = drag.current;
    if (!held0) return;
    setPlace(
      held({
        zoom: held0.from.zoom,
        x: held0.from.x + (event.clientX - held0.x),
        y: held0.from.y + (event.clientY - held0.y),
      }),
    );
  }

  function release(event: React.PointerEvent<HTMLDivElement>) {
    if (drag.current) event.currentTarget.releasePointerCapture(event.pointerId);
    drag.current = null;
  }

  async function use() {
    if (!source || busy) return;
    setBusy(true);
    try {
      onUse(await squared(source, place, file.name));
    } finally {
      setBusy(false);
    }
  }

  const covered = source ? cover(source, place.zoom) : { width: 0, height: 0 };
  const layer = {
    width: covered.width,
    height: covered.height,
    left: (STAGE_WIDTH - covered.width) / 2 + place.x,
    top: (STAGE_HEIGHT - covered.height) / 2 + place.y,
  };
  const preview = source ? crop(source, place) : null;

  return (
    <div className="scrim" onPointerDown={onCancel}>
      <div
        className="place-picture"
        role="dialog"
        aria-label="Place your picture"
        onPointerDown={(event) => event.stopPropagation()}
      >
        <div className="place-head">
          <span className="place-title">Place your picture</span>
          <span className="place-subtitle">
            what is inside the square is what everyone sees
          </span>
        </div>

        <div
          className="crop-stage"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={release}
          onPointerCancel={release}
        >
          {source && (
            <img className="crop-wide" src={source.src} alt="" style={layer} draggable={false} />
          )}
          <span className="crop-scrim" />
          <div className="crop-square">
            {source && (
              <img
                className="crop-close"
                src={source.src}
                alt=""
                draggable={false}
                style={{
                  width: layer.width,
                  height: layer.height,
                  left: layer.left - (STAGE_WIDTH - SQUARE) / 2,
                  top: layer.top - (STAGE_HEIGHT - SQUARE) / 2,
                }}
              />
            )}
          </div>
          <span className="crop-hint">drag to move</span>
        </div>

        <div className="zoom-row">
          <span className="zoom-sign">−</span>
          <input
            type="range"
            className="zoom-input"
            min={100}
            max={MAX_ZOOM * 100}
            value={Math.round(place.zoom * 100)}
            aria-label="Zoom"
            style={
              {
                "--fill": `${((place.zoom - 1) / (MAX_ZOOM - 1)) * 100}%`,
              } as React.CSSProperties
            }
            onChange={(event) =>
              setPlace((was) => held({ ...was, zoom: Number(event.target.value) / 100 }))
            }
          />
          <span className="zoom-sign">+</span>
          <span className="zoom-value">{place.zoom.toFixed(1)}×</span>
        </div>

        <div className="place-preview">
          {[44, 26, 20].map((size, i) => (
            <span
              key={size}
              className="place-preview-square"
              style={{
                width: size,
                height: size,
                borderRadius: [12, 7, 6][i],
                backgroundImage: preview ? `url(${preview})` : undefined,
              }}
            />
          ))}
          <span className="place-preview-text">
            how it lands beside your name, in a call, and in the roster
          </span>
        </div>

        <div className="place-actions">
          <Button kind="quiet" onClick={() => setPlace(CENTRED)}>
            Reset
          </Button>
          <div className="place-actions-right">
            <Button kind="quiet" onClick={onCancel}>
              Cancel
            </Button>
            <Button disabled={!source || busy} onClick={use}>
              Use this
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function cover(source: HTMLImageElement, zoom: number) {
  const scale = (Math.max(SQUARE / source.width, SQUARE / source.height) * zoom);
  return { width: source.width * scale, height: source.height * scale };
}

function paint(source: HTMLImageElement, place: Placement, side: number) {
  const canvas = document.createElement("canvas");
  canvas.width = side;
  canvas.height = side;
  const paper = canvas.getContext("2d");
  if (!paper) return null;
  paper.fillStyle = "#161826";
  paper.fillRect(0, 0, side, side);
  const covered = cover(source, place.zoom);
  const ratio = side / SQUARE;
  paper.drawImage(
    source,
    (SQUARE - covered.width) / 2 * ratio + place.x * ratio,
    (SQUARE - covered.height) / 2 * ratio + place.y * ratio,
    covered.width * ratio,
    covered.height * ratio,
  );
  return canvas;
}

function crop(source: HTMLImageElement, place: Placement): string | null {
  return paint(source, place, 64)?.toDataURL("image/png") ?? null;
}

function squared(source: HTMLImageElement, place: Placement, name: string): Promise<File> {
  return new Promise((resolve, reject) => {
    const canvas = paint(source, place, OUTPUT);
    if (!canvas) {
      reject(new Error("no canvas"));
      return;
    }
    canvas.toBlob(
      (blob) => {
        if (!blob) {
          reject(new Error("no blob"));
          return;
        }
        resolve(
          new File([blob], `${name.replace(/\.[^.]+$/, "")}.jpg`, { type: "image/jpeg" }),
        );
      },
      "image/jpeg",
      0.92,
    );
  });
}
