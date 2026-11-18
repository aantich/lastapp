import type { ReactNode } from "react";

export type Segment = { value: string; label: ReactNode };

// A few mutually exclusive views of one list ("All · People · Agents ·
// System"): a labelled group of toggle buttons, the pressed one is current.
// For a form choice use RadioGroup; for many options use Select.
export function SegmentedControl({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: Segment[];
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="ui-segmented" role="group" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={o.value === value}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
