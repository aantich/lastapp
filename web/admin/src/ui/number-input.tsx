import { useEffect, useRef, useState, type InputHTMLAttributes } from "react";
import { Minus, Plus } from "lucide-react";
// A text field for numbers: no native stepper, numeric keyboard on mobile,
// ArrowUp/ArrowDown step, out-of-range values clamp on blur and block submit.
export function NumberInput({
  value,
  onChange,
  min,
  max,
  step = 1,
  decimal = false,
  steppers = false,
  onBlur,
  onKeyDown,
  ...rest
}: Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "type" | "value" | "onChange" | "min" | "max" | "step" | "inputMode"
> & {
  value: number | string | null | undefined;
  onChange: (value: number | "") => void;
  min?: number;
  max?: number;
  step?: number;
  decimal?: boolean;
  steppers?: boolean;
}) {
  const input = useRef<HTMLInputElement>(null);
  // The raw text while the user edits; null shows the committed value.
  const [draft, setDraft] = useState<string | null>(null);
  const text = draft ?? (value === "" || value == null ? "" : String(value));
  const negative = min === undefined || min < 0;
  const allowed = new RegExp(
    "^" + (negative ? "-?" : "") + (decimal ? "\\d*\\.?\\d*$" : "\\d*$"),
  );
  const parse = (s: string): number | "" | null =>
    s.trim() === ""
      ? ""
      : allowed.test(s) && /\d/.test(s) && Number.isFinite(Number(s))
        ? Number(s)
        : null;
  const clamp = (n: number) =>
    Math.min(max ?? Infinity, Math.max(min ?? -Infinity, n));
  const parsed = parse(text);
  const message =
    parsed === null
      ? "Enter a number."
      : parsed !== "" && clamp(parsed) !== parsed
        ? min !== undefined && max !== undefined
          ? `Enter a number from ${min} to ${max}.`
          : min !== undefined
            ? `Enter ${min} or more.`
            : `Enter ${max} or less.`
        : "";
  useEffect(() => {
    input.current?.setCustomValidity(message);
  }, [message]);
  function nudge(direction: 1 | -1) {
    const base = typeof parsed === "number" ? parsed : (min ?? 0);
    const next = clamp(Number((base + direction * step).toFixed(10)));
    if (draft !== null) setDraft(String(next));
    onChange(next);
  }
  const field = (
    <input
      {...rest}
      ref={input}
      type="text"
      inputMode={decimal ? "decimal" : "numeric"}
      autoComplete="off"
      value={text}
      onChange={(e) => {
        const next = e.target.value.trim();
        if (next !== "" && !allowed.test(next)) return;
        setDraft(next);
        const n = parse(next);
        if (n !== null) onChange(n);
      }}
      onBlur={(e) => {
        setDraft(null);
        if (typeof parsed === "number" && clamp(parsed) !== parsed)
          onChange(clamp(parsed));
        onBlur?.(e);
      }}
      onKeyDown={(e) => {
        if (e.key === "ArrowUp" || e.key === "ArrowDown") {
          e.preventDefault();
          nudge(e.key === "ArrowUp" ? 1 : -1);
        }
        onKeyDown?.(e);
      }}
    />
  );
  if (!steppers) return field;
  const n = typeof parsed === "number" ? parsed : undefined;
  return (
    <span className="number-input">
      {field}
      <button
        type="button"
        className="secondary"
        tabIndex={-1}
        aria-label="Decrease"
        disabled={
          rest.disabled || (n !== undefined && min !== undefined && n <= min)
        }
        onClick={() => nudge(-1)}
      >
        <Minus size={14} />
      </button>
      <button
        type="button"
        className="secondary"
        tabIndex={-1}
        aria-label="Increase"
        disabled={
          rest.disabled || (n !== undefined && max !== undefined && n >= max)
        }
        onClick={() => nudge(1)}
      >
        <Plus size={14} />
      </button>
    </span>
  );
}
