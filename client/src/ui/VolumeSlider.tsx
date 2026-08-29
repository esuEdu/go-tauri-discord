export function VolumeSlider({
  label,
  value,
  max,
  accentValue,
  onChange,
}: {
  label: string;
  value: number;
  max: number;
  accentValue?: boolean;
  onChange: (value: number) => void;
}) {
  const percent = Math.round(value * 100);
  return (
    <div className="volume-slider">
      <div className="volume-caption">
        <span className="volume-label">{label}</span>
        <span className="volume-value" data-accent={accentValue}>
          {percent}%
        </span>
      </div>
      <div className="volume-track-wrap">
        <input
          className="volume-input"
          type="range"
          min={0}
          max={max * 100}
          step={1}
          value={percent}
          aria-label={label}
          style={{ "--fill": `${(value / max) * 100}%` } as React.CSSProperties}
          onChange={(event) => onChange(Number(event.target.value) / 100)}
        />
        <span className="volume-tick" />
      </div>
    </div>
  );
}
