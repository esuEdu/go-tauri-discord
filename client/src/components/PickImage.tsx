import { useRef, useState } from "react";
import { api, ApiError } from "../api";
import { Avatar } from "./Avatar";

export function PickImage({
  name,
  imageKey,
  label,
  onChosen,
  upload,
  remove,
}: {
  name: string;
  imageKey: string | null | undefined;
  label: string;
  onChosen: (key: string | null) => void;
  upload: (file: File) => Promise<string>;
  remove: () => Promise<void>;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);

  async function chose(file: File | undefined) {
    if (!file) return;
    setBusy(true);
    setProblem(null);
    try {
      onChosen(await upload(file));
    } catch (err) {
      setProblem(err instanceof ApiError ? err.message : "could not upload that image");
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
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
    <div className="pick-image">
      <Avatar name={name} imageKey={imageKey} className="avatar large" />

      <div className="pick-image-actions">
        <button className="link" disabled={busy} onClick={() => input.current?.click()}>
          {imageKey ? `Change ${label}` : `Add ${label}`}
        </button>
        {imageKey && (
          <button className="link danger" disabled={busy} onClick={() => void clear()}>
            Remove
          </button>
        )}
      </div>

      <input
        ref={input}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        hidden
        onChange={(e) => void chose(e.target.files?.[0])}
      />

      {problem && <div className="error">{problem}</div>}
    </div>
  );
}

export function AvatarPicker({
  name,
  imageKey,
  onChosen,
}: {
  name: string;
  imageKey: string | null | undefined;
  onChosen: (key: string | null) => void;
}) {
  return (
    <PickImage
      name={name}
      imageKey={imageKey}
      label="picture"
      onChosen={onChosen}
      upload={async (file) => (await api.setAvatar(file)).avatar_key}
      remove={() => api.clearAvatar()}
    />
  );
}
