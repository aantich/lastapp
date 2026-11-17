import { useId, useRef, type KeyboardEvent, type ReactNode } from "react";
import { cx } from "./utils";
import { Badge } from "./status";

export type TabItem = {
  id: string;
  label: ReactNode;
  count?: number;
  disabled?: boolean;
};

// A WAI-ARIA tablist with roving tabindex: ←/→ move between tabs (wrapping),
// Home/End jump, disabled tabs are skipped. Controlled: `value` is the
// selected tab and `onChange` selects. With activation="manual" arrows only
// move focus and Enter/Space select. Children render inside the tabpanel.
export function Tabs({
  label,
  items,
  value,
  onChange,
  activation = "auto",
  className,
  children,
}: {
  label: string;
  items: TabItem[];
  value: string;
  onChange: (id: string) => void;
  activation?: "auto" | "manual";
  className?: string;
  children?: ReactNode;
}) {
  const base = useId(),
    list = useRef<HTMLDivElement>(null);
  const tabId = (id: string) => `${base}-tab-${id}`,
    panelId = `${base}-panel`;
  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>, index: number) {
    const enabled = items
      .map((item, i) => (item.disabled ? -1 : i))
      .filter((i) => i >= 0);
    const at = enabled.indexOf(index);
    let next: number | undefined;
    if (e.key === "ArrowRight") next = enabled[(at + 1) % enabled.length];
    else if (e.key === "ArrowLeft")
      next = enabled[(at - 1 + enabled.length) % enabled.length];
    else if (e.key === "Home") next = enabled[0];
    else if (e.key === "End") next = enabled.at(-1);
    if (next === undefined) return;
    e.preventDefault();
    const tabs =
      list.current?.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    tabs?.[next]?.focus();
    if (activation === "auto") onChange(items[next].id);
  }
  return (
    <>
      <div
        ref={list}
        role="tablist"
        aria-label={label}
        className={cx("tabs ui-tabs", className)}
      >
        {items.map((item, i) => {
          const selected = item.id === value;
          return (
            <button
              key={item.id}
              type="button"
              role="tab"
              id={tabId(item.id)}
              aria-selected={selected}
              aria-controls={children === undefined ? undefined : panelId}
              tabIndex={selected ? 0 : -1}
              disabled={item.disabled}
              onClick={() => onChange(item.id)}
              onKeyDown={(e) => onKeyDown(e, i)}
            >
              {item.label}
              {item.count !== undefined && <Badge count>{item.count}</Badge>}
            </button>
          );
        })}
      </div>
      {children !== undefined && (
        <div
          role="tabpanel"
          id={panelId}
          aria-labelledby={tabId(value)}
          tabIndex={0}
          className="ui-tabpanel"
        >
          {children}
        </div>
      )}
    </>
  );
}
