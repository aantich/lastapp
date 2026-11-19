import { useCallback, useLayoutEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Check } from "lucide-react";
import type { Listbox, ListOption } from "./listbox";
import { place, portalHost } from "./popup-placement";
import { cx } from "./utils";

// The floating popup of every console list control (Select, RemoteSelect,
// ContextSwitcher): portalled out of the control so it never changes the
// layout around it or inherits its surface, placed by `place` and following
// scrolling and resizing. `header` sits above the options (a search field),
// `children` under them (paging status, "Load more"). The current option
// carries a check mark.
export function ListboxPopup<
  T extends ListOption & { description?: ReactNode },
>({
  list,
  label,
  options,
  value,
  choose,
  header,
  children,
  side = "bottom",
  minWidth = 0,
  maxHeight = 270,
  className,
}: {
  list: Listbox;
  label?: string;
  options: T[];
  value: string;
  choose: (option: T) => void;
  header?: ReactNode;
  children?: ReactNode;
  side?: "bottom" | "right";
  minWidth?: number;
  maxHeight?: number;
  className?: string;
}) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null),
    [popup, setPopupElement] = useState<HTMLElement | null>(null);
  const { setPopup } = list;
  // The placeholder stays inside the control; its parent is the anchor.
  const placeholder = useCallback(
    (el: HTMLElement | null) => setAnchor(el?.parentElement ?? null),
    [],
  );
  const popupRef = useCallback(
    (el: HTMLElement | null) => {
      setPopupElement(el);
      setPopup(el);
    },
    [setPopup],
  );
  useLayoutEffect(() => {
    if (!popup || !anchor) return;
    const update = () => place(popup, anchor, side, minWidth, maxHeight);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(popup);
    observer.observe(anchor);
    addEventListener("scroll", update, true);
    addEventListener("resize", update);
    return () => {
      observer.disconnect();
      removeEventListener("scroll", update, true);
      removeEventListener("resize", update);
    };
  }, [popup, anchor, side, minWidth, maxHeight]);
  return (
    <>
      <span hidden ref={placeholder} />
      {anchor &&
        createPortal(
          <span
            className={cx("combobox-popup", className)}
            ref={popupRef}
            // Keep focus on the control (a search field keeps its own), and
            // stop an enclosing <label> from re-dispatching the click to it.
            onMouseDown={(e) => {
              if (!(e.target as HTMLElement).closest("input"))
                e.preventDefault();
            }}
            onClick={(e) => e.preventDefault()}
          >
            {header}
            <span role="listbox" id={list.id} aria-label={label}>
              {options.map((o, i) => (
                <span
                  role="option"
                  id={`${list.id}-${i}`}
                  key={o.value}
                  data-value={o.value}
                  aria-selected={value === o.value}
                  aria-disabled={o.disabled || undefined}
                  className={list.active === i ? "focused" : undefined}
                  onClick={() => {
                    if (!o.disabled) choose(o);
                  }}
                  onMouseMove={() => list.setActive(i)}
                >
                  <span>
                    {o.label}
                    {o.description && <small>{o.description}</small>}
                  </span>
                  {value === o.value && <Check size={15} aria-hidden />}
                </span>
              ))}
            </span>
            {children}
          </span>,
          portalHost(anchor),
        )}
    </>
  );
}
