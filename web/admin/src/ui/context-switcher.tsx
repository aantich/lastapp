import { useMemo, useRef, useState, type ReactNode } from "react";
import { Check, ChevronsUpDown, Search } from "lucide-react";
import { useListbox, type ListOption } from "./listbox";

export type ContextOption = ListOption & { description?: ReactNode };

// Switches the working Tenant or Space. The trigger shows the kind and the
// current name; it opens a popover with a filter field (a combobox) over a
// listbox: ↑/↓ move, Enter chooses, Escape closes and returns focus to the
// trigger, a press outside closes. Shares keyboard state with Select
// (useListbox).
export function ContextSwitcher({
  kind,
  value,
  options,
  onChange,
  placeholder = "Choose…",
  empty = "Nothing matches.",
}: {
  // "Tenant" or "Space": the eyebrow and the accessible names.
  kind: string;
  value: string | undefined;
  options: ContextOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  empty?: ReactNode;
}) {
  const root = useRef<HTMLDivElement>(null),
    trigger = useRef<HTMLButtonElement>(null),
    [query, setQuery] = useState("");
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q
      ? options.filter((o) => o.label.toLowerCase().includes(q))
      : options;
  }, [options, query]);
  const list = useListbox(shown, choose, root);
  const current = options.find((o) => o.value === value);
  function choose(option: ContextOption) {
    onChange(option.value);
    close();
  }
  function close() {
    list.setOpen(false);
    setQuery("");
    trigger.current?.focus();
  }
  function open() {
    const index = options.findIndex((o) => o.value === value);
    list.setActive(index >= 0 ? index : 0);
    list.setOpen(true);
  }
  return (
    <div className="ui-context" ref={root}>
      <button
        ref={trigger}
        type="button"
        className="ui-context-trigger"
        aria-haspopup="listbox"
        aria-expanded={list.open}
        aria-label={`${kind}: ${current?.label || placeholder}. Switch ${kind}`}
        onClick={() => (list.open ? close() : open())}
        onKeyDown={(e) => {
          if (!list.open && (e.key === "ArrowDown" || e.key === "ArrowUp")) {
            e.preventDefault();
            open();
          }
        }}
      >
        <span>
          <small>{kind}</small>
          <strong>{current?.label || placeholder}</strong>
        </span>
        <ChevronsUpDown size={16} aria-hidden />
      </button>
      {list.open && (
        <div className="ui-context-popup">
          <label className="ui-palette-input ui-context-filter">
            <Search size={15} aria-hidden />
            <input
              autoFocus
              role="combobox"
              aria-label={`Find a ${kind}`}
              aria-expanded
              aria-controls={list.id}
              aria-autocomplete="list"
              aria-activedescendant={list.activeId}
              placeholder={`Find a ${kind}…`}
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                list.setActive(0);
              }}
              onKeyDown={(e) => {
                if (e.key === "Escape") {
                  e.preventDefault();
                  e.stopPropagation();
                  close();
                  return;
                }
                list.onKeyDown(e);
              }}
            />
          </label>
          <div
            role="listbox"
            id={list.id}
            aria-label={`${kind}s`}
            className="ui-option-list"
          >
            {shown.map((option, i) => (
              <div
                key={option.value}
                id={`${list.id}-${i}`}
                role="option"
                data-value={option.value}
                aria-selected={option.value === value}
                aria-disabled={option.disabled || undefined}
                data-active={list.active === i || undefined}
                className="ui-option"
                onMouseDown={(e) => e.preventDefault()}
                onMouseMove={() => list.setActive(i)}
                onClick={() => {
                  if (!option.disabled) choose(option);
                }}
              >
                <span>
                  {option.label}
                  {option.description && <small>{option.description}</small>}
                </span>
                {option.value === value && <Check size={15} aria-hidden />}
              </div>
            ))}
            {!shown.length && <div className="ui-option-empty">{empty}</div>}
          </div>
        </div>
      )}
    </div>
  );
}
