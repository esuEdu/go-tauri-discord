import { Icon } from "./Icon";

export type Stance = "deny" | "inherit" | "allow";

const ORDER: Stance[] = ["deny", "inherit", "allow"];

const WORD: Record<Stance, string> = {
  deny: "Deny",
  inherit: "Inherit",
  allow: "Allow",
};

export function TriState({
  value,
  label,
  disabled,
  onChange,
}: {
  value: Stance;
  label: string;
  disabled?: boolean;
  onChange: (next: Stance) => void;
}) {
  return (
    <span className="tristate" role="radiogroup" aria-label={label}>
      {ORDER.map((stance) => (
        <button
          key={stance}
          type="button"
          className="tristate-option"
          role="radio"
          aria-checked={value === stance}
          aria-label={`${WORD[stance]} — ${label}`}
          title={WORD[stance]}
          data-stance={stance}
          data-on={value === stance}
          disabled={disabled}
          onClick={() => onChange(stance)}
        >
          {stance === "deny" && <Icon name="x" size={13} />}
          {stance === "inherit" && <span className="tristate-dash" />}
          {stance === "allow" && <Icon name="check" size={13} />}
        </button>
      ))}
    </span>
  );
}
