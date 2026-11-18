import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown } from "lucide-react";
import { useListbox, type Listbox, type ListOption } from "./listbox";
import { cx } from "./utils";
export type { ListOption } from "./listbox";

const gap = 4,
  margin = 8;
// The element a popup is portalled into: the open modal dialog that holds
// the control (so its focus trap and aria-modal still include the popup),
// otherwise <body>. The popup never takes part in the control's layout. The
// ink sidebar (a modal drawer on narrow screens) is never the host: a popup
// opened from it uses the page's surface tokens, not the sidebar's.
function portalHost(anchor: HTMLElement) {
  return (
    anchor.closest<HTMLElement>('[aria-modal="true"]:not(.sidebar)') ||
    document.body
  );
}
// Floats the popup over the page beside its anchor: below it, or above when
// the viewport has more room there ("right": beside a rail tile), at least
// as wide as the anchor and never taller than the space available; the list
// inside it scrolls.
function place(
  popup: HTMLElement,
  anchor: HTMLElement,
  side: "bottom" | "right",
  minWidth: number,
  cap: number,
) {
  const r = anchor.getBoundingClientRect(),
    width = Math.min(Math.max(r.width, minWidth), innerWidth - 2 * margin);
  popup.style.width = width + "px";
  let left: number,
    top: number,
    up = false;
  if (side === "right") {
    popup.style.maxHeight = Math.min(cap, innerHeight - 2 * margin) + "px";
    left = r.right + margin;
    top = Math.max(
      margin,
      Math.min(r.top, innerHeight - margin - popup.offsetHeight),
    );
  } else {
    const below = innerHeight - r.bottom - gap - margin,
      above = r.top - gap - margin;
    up = below < 200 && above > below;
    popup.style.maxHeight =
      Math.max(96, Math.min(cap, up ? above : below)) + "px";
    left = r.left;
    top = up ? r.top - gap - popup.offsetHeight : r.bottom + gap;
  }
  left = Math.max(margin, Math.min(left, innerWidth - margin - width));
  // A fixed popup inside a transformed host (the narrow-screen drawer) is
  // placed relative to that host.
  const host = popup.parentElement;
  if (
    host &&
    host !== document.body &&
    getComputedStyle(host).transform !== "none"
  ) {
    const h = host.getBoundingClientRect();
    left += host.scrollLeft - h.left;
    top += host.scrollTop - h.top;
  }
  popup.classList.toggle("above", up);
  popup.style.left = left + "px";
  popup.style.top = top + "px";
}

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
// Text of the <span> that opens the enclosing <label>, the console's labelling
// convention; used only to name the popup list.
function labelText(el: HTMLElement | null) {
  const first = (el as HTMLButtonElement | null)?.labels?.[0]
    ?.firstElementChild;
  return first?.tagName === "SPAN"
    ? first.firstChild?.textContent?.trim()
    : undefined;
}
export function Select({
  options,
  value,
  onChange,
  placeholder = "Choose…",
  disabled,
  required,
  name,
  id,
  className,
  "aria-label": ariaLabel,
}: {
  options: ListOption[];
  value: string | undefined;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  name?: string;
  id?: string;
  className?: string;
  "aria-label"?: string;
}) {
  const root = useRef<HTMLSpanElement>(null),
    trigger = useRef<HTMLButtonElement>(null),
    typed = useRef({ text: "", at: 0 });
  const list = useListbox(options, choose, root);
  const [listLabel, setListLabel] = useState<string>();
  const current = options.findIndex((o) => o.value === value),
    selected = options[current];
  function choose(o: ListOption) {
    onChange(o.value);
    list.setOpen(false);
  }
  function show(active = current) {
    setListLabel(ariaLabel ?? labelText(trigger.current));
    list.setActive(active >= 0 ? active : list.step(-1, 1));
    list.setOpen(true);
  }
  function typeahead(key: string) {
    const now = Date.now(),
      t = typed.current;
    t.text = (now - t.at < 700 ? t.text : "") + key.toLowerCase();
    t.at = now;
    const start = list.open ? list.active : Math.max(0, current);
    const order = options.map(
      (_, i) => (start + (t.text.length > 1 ? 0 : 1) + i) % options.length,
    );
    const match = order.find(
      (i) =>
        !options[i].disabled &&
        options[i].label.toLowerCase().startsWith(t.text),
    );
    if (match !== undefined) show(match);
  }
  return (
    <span
      className={["search-combobox select-control", className]
        .filter(Boolean)
        .join(" ")}
      ref={root}
    >
      <button
        type="button"
        ref={trigger}
        id={id}
        role="combobox"
        className="select-trigger"
        data-value={value ?? ""}
        aria-haspopup="listbox"
        aria-expanded={list.open}
        aria-controls={list.id}
        aria-activedescendant={list.activeId}
        aria-label={ariaLabel}
        aria-required={required || undefined}
        disabled={disabled}
        onClick={() => (list.open ? list.setOpen(false) : show())}
        onBlur={(e) => {
          if (!root.current?.contains(e.relatedTarget)) list.setOpen(false);
        }}
        onKeyDown={(e) => {
          const t = typed.current,
            typing = Date.now() - t.at < 700 && t.text !== "";
          if (
            e.key.length === 1 &&
            !e.ctrlKey &&
            !e.metaKey &&
            !e.altKey &&
            (e.key !== " " || typing)
          ) {
            e.preventDefault();
            typeahead(e.key);
            return;
          }
          if (
            !list.open &&
            ["Enter", " ", "ArrowDown", "ArrowUp"].includes(e.key)
          ) {
            e.preventDefault();
            show();
            return;
          }
          if (list.onKeyDown(e) || !list.open) return;
          if (e.key === "Home" || e.key === "End") {
            e.preventDefault();
            list.setActive(
              e.key === "Home"
                ? list.step(-1, 1)
                : list.step(options.length, -1),
            );
          } else if (e.key === " ") {
            e.preventDefault();
            const o = options[list.active];
            if (o && !o.disabled) choose(o);
          }
        }}
      >
        <span
          className={selected ? "select-value" : "select-value placeholder"}
        >
          {selected?.label ?? placeholder}
        </span>
        <ChevronDown size={15} aria-hidden="true" />
      </button>
      {(required || name) && (
        // Carries the value for constraint validation and form data; the
        // trigger above is the labelled, focusable control.
        <input
          className="select-native-value"
          tabIndex={-1}
          aria-hidden="true"
          name={name}
          required={required}
          disabled={disabled}
          value={value ?? ""}
          onChange={() => {}}
          onFocus={() => trigger.current?.focus()}
        />
      )}
      {list.open && (
        <ListboxPopup
          list={list}
          label={listLabel}
          options={options}
          value={value ?? ""}
          choose={choose}
        />
      )}
    </span>
  );
}
