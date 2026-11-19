import { useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { useListbox, type ListOption } from "./listbox";
import { ListboxPopup } from "./listbox-popup";
export type { ListOption } from "./listbox";
export { ListboxPopup } from "./listbox-popup";

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
