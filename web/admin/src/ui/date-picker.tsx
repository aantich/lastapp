import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { cx } from "./utils";
import { addDays, addMonths, daysBetween, isoDay, parseDay } from "./days";
import { longDate } from "./long-date";
import { CalendarGrid } from "./calendar-grid";

// A date field: a button showing the chosen day that opens a calendar dialog
// (WAI-ARIA date picker dialog: arrows move a day/week, PageUp/PageDown a
// month, Home/End the week's ends, Enter/Space choose, Escape closes). Days
// outside min/max are disabled. It replaces the native date input, which the
// console does not use. Wrap it in FormField for its label.
export function DatePicker({
  value,
  onChange,
  min,
  max,
  id,
  disabled,
  placeholder = "Choose a date",
  "aria-label": ariaLabel,
  "aria-describedby": describedBy,
  "aria-invalid": invalid,
}: {
  value: string;
  onChange: (value: string) => void;
  min?: string;
  max?: string;
  id?: string;
  disabled?: boolean;
  placeholder?: string;
  "aria-label"?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  // FormField passes it; a button cannot carry aria-required, so a
  // required date is enforced by the caller (the picker always has a value).
  "aria-required"?: boolean;
}) {
  const selected = parseDay(value),
    lo = parseDay(min),
    hi = parseDay(max);
  const [open, setOpen] = useState(false);
  const [focus, setFocus] = useState<Date>(() => selected ?? new Date());
  const root = useRef<HTMLSpanElement>(null),
    trigger = useRef<HTMLButtonElement>(null),
    grid = useRef<HTMLTableElement>(null);
  const dialogId = useId(),
    titleId = useId();
  const allowed = (d: Date) =>
    (!lo || daysBetween(lo, d) >= 0) && (!hi || daysBetween(d, hi) >= 0);
  const clamp = (d: Date) =>
    lo && daysBetween(lo, d) < 0 ? lo : hi && daysBetween(d, hi) < 0 ? hi : d;

  function show() {
    setFocus(clamp(selected ?? new Date()));
    setOpen(true);
  }
  function close(returnFocus = true) {
    setOpen(false);
    if (returnFocus) trigger.current?.focus();
  }
  function choose(d: Date) {
    if (!allowed(d)) return;
    onChange(isoDay(d));
    close();
  }
  // Keep focus on the focused day while the dialog is open.
  useEffect(() => {
    if (!open) return;
    grid.current
      ?.querySelector<HTMLButtonElement>(`[data-day="${isoDay(focus)}"]`)
      ?.focus();
  }, [open, focus]);
  // A click outside closes without choosing.
  useEffect(() => {
    if (!open) return;
    const outside = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", outside);
    return () => document.removeEventListener("mousedown", outside);
  }, [open]);

  function key(e: KeyboardEvent) {
    const moves: Record<string, (d: Date) => Date> = {
      ArrowLeft: (d) => addDays(d, -1),
      ArrowRight: (d) => addDays(d, 1),
      ArrowUp: (d) => addDays(d, -7),
      ArrowDown: (d) => addDays(d, 7),
      PageUp: (d) => addMonths(d, e.shiftKey ? -12 : -1),
      PageDown: (d) => addMonths(d, e.shiftKey ? 12 : 1),
      Home: (d) => addDays(d, -((d.getDay() + 6) % 7)),
      End: (d) => addDays(d, 6 - ((d.getDay() + 6) % 7)),
    };
    if (moves[e.key]) {
      e.preventDefault();
      setFocus(clamp(moves[e.key](focus)));
    }
  }

  const month = focus.toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
  });
  const prev = addMonths(focus, -1),
    next = addMonths(focus, 1);
  const prevAllowed =
      !lo ||
      daysBetween(lo, new Date(focus.getFullYear(), focus.getMonth(), 0)) >= 0,
    nextAllowed =
      !hi ||
      daysBetween(new Date(next.getFullYear(), next.getMonth(), 1), hi) >= 0;

  return (
    <span className="ui-date" ref={root}>
      <button
        type="button"
        ref={trigger}
        id={id}
        className="ui-date-trigger"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? dialogId : undefined}
        aria-label={ariaLabel}
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        data-value={value}
        disabled={disabled}
        onClick={() => (open ? close() : show())}
      >
        <span className={cx(!selected && "ui-date-placeholder")}>
          {selected ? longDate(selected) : placeholder}
        </span>
        <CalendarDays size={15} aria-hidden />
      </button>
      {open && (
        <div
          className="ui-date-popup"
          role="dialog"
          id={dialogId}
          aria-modal="false"
          aria-labelledby={titleId}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault();
              e.stopPropagation();
              close();
            }
          }}
        >
          <div className="ui-date-head">
            <button
              type="button"
              className="ui-date-nav"
              aria-label="Previous month"
              disabled={!prevAllowed}
              onClick={() => setFocus(clamp(prev))}
            >
              <ChevronLeft size={16} aria-hidden />
            </button>
            <span className="ui-date-title" id={titleId} aria-live="polite">
              {month}
            </span>
            <button
              type="button"
              className="ui-date-nav"
              aria-label="Next month"
              disabled={!nextAllowed}
              onClick={() => setFocus(clamp(next))}
            >
              <ChevronRight size={16} aria-hidden />
            </button>
          </div>
          <CalendarGrid
            focus={focus}
            value={value}
            titleId={titleId}
            gridRef={grid}
            allowed={allowed}
            choose={choose}
            setFocus={setFocus}
            onKeyDown={key}
          />
        </div>
      )}
    </span>
  );
}
