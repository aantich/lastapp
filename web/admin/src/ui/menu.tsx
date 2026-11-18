import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import { cx } from "./utils";

export type MenuItem = {
  id: string;
  label: string;
  icon?: ReactNode;
  tone?: "default" | "danger";
  onSelect: () => void;
};

// A WAI-ARIA menu button: the trigger opens a menu of actions (account,
// row actions). ↓/↑ (wrapping), Home and End move between items, Enter or
// Space runs one, Escape closes and returns focus to the trigger, Tab or a
// press outside closes. Focus is back on the trigger before an item runs, so
// a dialog the item opens returns focus there. The trigger's accessible name
// is its content: add hidden words with .ui-sr-only rather than a `label`
// that repeats the visible text (WCAG 2.5.3 label in name). `label` is for
// an icon-only trigger.
export function Menu({
  trigger,
  label,
  items,
  align = "end",
  placement = "below",
  className,
  triggerClassName,
}: {
  trigger: ReactNode;
  label?: string;
  items: MenuItem[];
  align?: "start" | "end";
  // "above" opens upward (a trigger at the bottom of the sidebar).
  placement?: "below" | "above";
  className?: string;
  triggerClassName?: string;
}) {
  const id = useId(),
    root = useRef<HTMLDivElement>(null),
    button = useRef<HTMLButtonElement>(null),
    menu = useRef<HTMLDivElement>(null),
    [open, setOpen] = useState(false);
  const entries = () =>
    Array.from(
      menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') || [],
    );
  useEffect(() => {
    if (!open) return;
    entries()[0]?.focus();
    const outside = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, [open]);
  function close() {
    setOpen(false);
    button.current?.focus();
  }
  function onKeyDown(e: KeyboardEvent) {
    const all = entries(),
      at = all.indexOf(document.activeElement as HTMLElement);
    const next =
      e.key === "ArrowDown"
        ? (at + 1) % all.length
        : e.key === "ArrowUp"
          ? (at - 1 + all.length) % all.length
          : e.key === "Home"
            ? 0
            : e.key === "End"
              ? all.length - 1
              : -1;
    if (next >= 0) {
      e.preventDefault();
      all[next]?.focus();
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close();
    } else if (e.key === "Tab") setOpen(false);
  }
  return (
    <div className={cx("ui-menu", className)} ref={root}>
      <button
        ref={button}
        id={id + "-trigger"}
        type="button"
        className={cx("ui-menu-trigger", triggerClassName)}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        onClick={() => setOpen(!open)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            setOpen(true);
          }
        }}
      >
        {trigger}
      </button>
      {open && (
        <div
          ref={menu}
          id={id}
          role="menu"
          aria-labelledby={id + "-trigger"}
          className={cx(
            "ui-menu-popup",
            align === "start" && "ui-start",
            placement === "above" && "ui-above",
          )}
          onKeyDown={onKeyDown}
        >
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              role="menuitem"
              tabIndex={-1}
              className={cx(item.tone === "danger" && "danger")}
              onClick={() => {
                close();
                item.onSelect();
              }}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
