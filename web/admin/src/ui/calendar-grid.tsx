import type { KeyboardEvent, RefObject } from "react";
import { cx } from "./utils";
import { addDays, isoDay } from "./days";
import { longDate } from "./long-date";

const weekdays = Array.from(
  { length: 7 },
  (_, i) =>
    // 2024-01-01 is a Monday; weeks start on Monday.
    new Date(2024, 0, 1 + i),
);

// The DatePicker's month grid: weekday headers over six weeks of day
// buttons; the focused day is the one tab stop.
export function CalendarGrid({
  focus,
  value,
  titleId,
  gridRef,
  allowed,
  choose,
  setFocus,
  onKeyDown,
}: {
  focus: Date;
  value: string;
  titleId: string;
  gridRef: RefObject<HTMLTableElement | null>;
  allowed: (d: Date) => boolean;
  choose: (d: Date) => void;
  setFocus: (d: Date) => void;
  onKeyDown: (e: KeyboardEvent) => void;
}) {
  // Six Monday-first weeks covering the focused month.
  const first = new Date(focus.getFullYear(), focus.getMonth(), 1),
    start = addDays(first, -((first.getDay() + 6) % 7));
  const weeks = Array.from({ length: 6 }, (_, w) =>
    Array.from({ length: 7 }, (_, d) => addDays(start, w * 7 + d)),
  );
  return (
    <table
      className="ui-date-grid"
      role="grid"
      ref={gridRef}
      aria-labelledby={titleId}
      onKeyDown={onKeyDown}
    >
      <thead>
        <tr>
          {weekdays.map((d) => (
            <th
              key={d.getDay()}
              scope="col"
              abbr={d.toLocaleDateString(undefined, { weekday: "long" })}
            >
              {d.toLocaleDateString(undefined, { weekday: "short" })}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {weeks.map((week) => (
          <tr key={isoDay(week[0])}>
            {week.map((d) => {
              const day = isoDay(d),
                inMonth = d.getMonth() === focus.getMonth(),
                ok = allowed(d);
              return (
                <td key={day} aria-selected={day === value}>
                  <button
                    type="button"
                    data-day={day}
                    tabIndex={day === isoDay(focus) ? 0 : -1}
                    aria-label={longDate(d)}
                    aria-current={
                      day === isoDay(new Date()) ? "date" : undefined
                    }
                    aria-disabled={!ok || undefined}
                    className={cx(!inMonth && "ui-date-outside")}
                    onClick={() => (ok ? choose(d) : setFocus(d))}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        choose(d);
                      }
                    }}
                  >
                    {d.getDate()}
                  </button>
                </td>
              );
            })}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
