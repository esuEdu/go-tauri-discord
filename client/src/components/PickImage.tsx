import { useRef, useState } from "react";
import { ApiError, type Progress } from "../api";
import { Avatar } from "./Avatar";
import { CropImage } from "./CropImage";
import { Icon } from "./Icon";

const ACCEPTS = "image/png,image/jpeg,image/gif,image/webp";
const MAX_BYTES = 5 << 20;
const MAX_PIXELS = 24_000_000;

export function PickImage({
  name,
  imageKey,
  label,
  onChosen,
  upload,
  remove,
  className,
  mine = false,
}: {
  name: string;
  imageKey: string | null | undefined;
  label: string;
  onChosen: (key: string | null) => void;
  upload: (file: File, onProgress: Progress) => Promise<string>;
  remove: () => Promise<void>;
  className?: string;
  mine?: boolean;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(0);
  const [problem, setProblem] = useState<string | null>(null);
  const [placing, setPlacing] = useState<File | null>(null);

  async function chose(file: File | undefined) {
    if (input.current) input.current.value = "";
    if (!file) return;
    setProblem(null);

    if (file.size > MAX_BYTES) {
      setProblem(
        `That one is ${(file.size / (1 << 20)).toFixed(1)} MB — the limit is 5 MB. Also refused: ` +
          "anything we cannot read as an image, or over 24 megapixels however small the file.",
      );
      return;
    }

    const measured = await new Promise<HTMLImageElement | null>((resolve) => {
      const url = URL.createObjectURL(file);
      const img = new Image();
      img.onload = () => {
        URL.revokeObjectURL(url);
        resolve(img);
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        resolve(null);
      };
      img.src = url;
    });

    if (!measured) {
      setProblem("We cannot read that as an image.");
      return;
    }
    if (measured.naturalWidth * measured.naturalHeight > MAX_PIXELS) {
      setProblem("That one is over 24 megapixels, however small the file is.");
      return;
    }

    setPlacing(file);
  }

  async function send(cropped: File) {
    setPlacing(null);
    setBusy(true);
    setSent(0);
    setProblem(null);
    try {
      onChosen(await upload(cropped, setSent));
    } catch (err) {
      setProblem(err instanceof ApiError ? err.message : "could not upload that image");
    } finally {
      setBusy(false);
    }
  }

  async function clear() {
    setBusy(true);
    setProblem(null);
    try {
      await remove();
      onChosen(null);
    } catch (err) {
      setProblem(err instanceof ApiError ? err.message : "could not remove that image");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="stack tight">
      {placing && (
        <CropImage
          file={placing}
          title={`Place ${label === "a picture" ? "your picture" : label}`}
          onCancel={() => setPlacing(null)}
          onDone={(cropped) => void send(cropped)}
        />
      )}

      <div className="row top">
        {busy ? (
          <span className={["avatar", "sending", className].filter(Boolean).join(" ")}>
            <span className="spinner" />
          </span>
        ) : (
          <Avatar name={name} imageKey={imageKey} className={className} mine={mine} />
        )}

        <div className="grow stack tight">
          {busy ? (
            <>
              <div className="panel-label">Sending… {Math.round(sent * 100)}%</div>
              <span className="upload-bar">
                <span style={{ width: `${Math.round(sent * 100)}%` }} />
              </span>
            </>
          ) : (
            <div className="row">
              <button className="btn btn-primary btn-small" onClick={() => input.current?.click()}>
                {imageKey ? "Change" : "Add"} {label}
              </button>
              {imageKey && (
                <button className="btn btn-quiet btn-small" onClick={() => void clear()}>
                  Remove
                </button>
              )}
            </div>
          )}
          <span className="field-note">
            5 MB and 24 megapixels at most. The middle is kept, squared, shrunk to 256px.
          </span>
        </div>
      </div>

      <input
        ref={input}
        type="file"
        accept={ACCEPTS}
        hidden
        onChange={(e) => void chose(e.target.files?.[0])}
      />

      {problem && (
        <div className="banner bad">
          <Icon name="warning-circle" size={16} />
          <span>{problem}</span>
        </div>
      )}
    </div>
  );
}
