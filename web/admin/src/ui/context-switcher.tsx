import { useMemo, useRef, useState, type ReactNode } from "react";
import { ChevronsUpDown, Search } from "lucide-react";
import { text, type Item } from "../api-types";
import { useListbox, type ListOption } from "./listbox";
import { useRemoteOptions } from "./remote-options";
import { RemoteListStatus } from "./remote-select";
import { ListboxPopup } from "./select";
import { Tooltip } from "./tooltip";
import { cx, initials } from "./utils";

export type ContextOption = ListOption & { description?: ReactNode };

const byName = (items: Item[]): ContextOption[] =>
  items.map((x) => ({ value: text(x.id), label: text(x.display_name) }));

// Switches the working Tenant or Space: a searchable select. The trigger
// shows the kind and the current name; it opens the kit's floating
// ListboxPopup (portalled over the page, below the trigger or beside the
// rail tile) with a filter field (a combobox) over the listbox: ↑/↓ move,
// Enter chooses, Escape closes and returns focus to the trigger, a press
// outside closes; typing on the trigger opens it and starts the search.
// Shares keyboard state with Select (useListbox). With `path` the choices
// come from that management list API: the filter searches on the server and
// further pages load inside the popup (the paging RemoteSelect uses);
// `options` are then the choices shown before the first search and
// `current` names a value not among them.
export function ContextSwitcher({
  kind,
  value,
  options,
  onChange,
  path,
  convert = byName,
  current: currentLabel,
  placeholder = "Choose…",
  empty,
  variant = "field",
  rail,
  className,
}: {
  // "Tenant" or "Space": the caption and the accessible names.
  kind: string;
  value: string | undefined;
  options: ContextOption[];
  onChange: (value: string) => void;
  path?: string;
  convert?: (items: Item[]) => ContextOption[];
  current?: string;
  placeholder?: string;
  empty?: ReactNode;
  // "sidebar": the ink sidebar's switcher under the brand row (initials
  // tile, caption, name and chevron); on the collapsed `rail` only the tile
  // shows, with a tooltip, and the popup opens beside it.
  variant?: "field" | "sidebar";
  rail?: boolean;
  className?: string;
}) {
  const root = useRef<HTMLDivElement>(null),
    trigger = useRef<HTMLButtonElement>(null);
  // Owned here because the remote list loads only while the popup is open.
  const openState = useState(false);
  const remote = useRemoteOptions({
    path: path || "",
    open: openState[0] && !!path,
    convert,
    initial: options,
  });
  const { query, setQuery } = remote;
  const shown = useMemo(() => {
    if (path) return remote.options;
    const q = query.trim().toLowerCase();
    return q
      ? options.filter((o) => o.label.toLowerCase().includes(q))
      : options;
  }, [path, remote.options, options, query]);
  const list = useListbox(shown, choose, root, openState);
  const name =
    (
      options.find((o) => o.value === value) ||
      remote.options.find((o) => o.value === value)
    )?.label ||
    currentLabel ||
    placeholder;
  const noMatch = empty ?? `No ${kind}s match`;
  function choose(option: ContextOption) {
    onChange(option.value);
    close();
  }
  function close() {
    list.setOpen(false);
    remote.reset();
    trigger.current?.focus();
  }
  function open(search = "") {
    remote.reset();
    if (search) setQuery(search);
    const index = search ? 0 : shown.findIndex((o) => o.value === value);
    list.setActive(index >= 0 ? index : 0);
    list.setOpen(true);
  }
  const sidebar = variant === "sidebar";
  const button = (
    <button
      ref={trigger}
      type="button"
      className={sidebar ? "ui-switcher-trigger" : "ui-context-trigger"}
      aria-haspopup="listbox"
      aria-expanded={list.open}
      onClick={() => (list.open ? close() : open())}
      onKeyDown={(e) => {
        if (list.open) return;
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          open();
        } else if (
          e.key.length === 1 &&
          e.key !== " " &&
          !e.ctrlKey &&
          !e.metaKey &&
          !e.altKey
        ) {
          e.preventDefault();
          open(e.key);
        }
      }}
    >
      {sidebar && (
        <span className="ui-switcher-tile" aria-hidden>
          {initials(name)}
        </span>
      )}
      {/* The accessible name is the visible caption and name ("Tenant
          Example Co, switch"); on the rail they stay for screen readers. */}
      <span className={sidebar ? "ui-switcher-copy" : undefined}>
        <small>{kind}</small> <strong>{name}</strong>
        {sidebar && <span className="ui-sr-only">, switch</span>}
      </span>
      <ChevronsUpDown size={sidebar ? 15 : 16} aria-hidden />
    </button>
  );
  return (
    <div
      className={cx("ui-context", sidebar && "ui-switcher", className)}
      ref={root}
    >
      {rail ? (
        <Tooltip
          content={`Switch ${kind} · ${name}`}
          side="right"
          labelOnly
          disabled={list.open}
        >
          {button}
        </Tooltip>
      ) : (
        button
      )}
      {list.open && (
        <ListboxPopup
          list={list}
          label={`${kind}s`}
          options={shown}
          value={value ?? ""}
          choose={choose}
          side={rail ? "right" : "bottom"}
          minWidth={260}
          maxHeight={380}
          className="ui-context-popup"
          header={
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
          }
        >
          {path ? (
            <RemoteListStatus
              remote={remote}
              count={shown.length}
              reveal={list.setActive}
              empty={noMatch}
            />
          ) : (
            !shown.length && <small>{noMatch}</small>
          )}
        </ListboxPopup>
      )}
    </div>
  );
}
