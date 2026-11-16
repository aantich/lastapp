import { useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { api } from "./api";
import type { Item, Page } from "./api-types";
import { useListbox, type ListOption as Option } from "./listbox";
import { ListboxPopup } from "./select";
export function RemoteSelect({
  path,
  label,
  value,
  onChange,
  initial,
  required,
  valueKey = "id",
  labelKey = "display_name",
  excludeInactive = false,
  excludeControl = false,
  excludePlatform = false,
}: {
  path: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  initial: Option[];
  required?: boolean;
  valueKey?: string;
  labelKey?: string;
  excludeInactive?: boolean;
  excludeControl?: boolean;
  excludePlatform?: boolean;
}) {
  const root = useRef<HTMLSpanElement>(null),
    input = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState(""),
    [page, setPage] = useState(1),
    [options, setOptions] = useState<Option[]>(initial),
    [selected, setSelected] = useState<Option | undefined>(
      initial.find((o) => o.value === value),
    ),
    [more, setMore] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const list = useListbox(options, choose, root),
    { open, setOpen, setActive } = list;
  const convert = (items: Item[]) =>
    items
      .filter(
        (x) =>
          (!excludeControl || x.domain !== "control") &&
          (!excludePlatform || !x.platform_admin) &&
          (!excludeInactive ||
            [x.state, x.user_state, x.tenant_state].every(
              (v) => !v || v === "active",
            )),
      )
      .map((x) => ({
        value: x[valueKey],
        label:
          String(x[labelKey] || "") +
          (x.email && labelKey === "display_name" ? ` (${x.email})` : ""),
      }));
  useEffect(() => {
    input.current?.setCustomValidity(
      required && !value ? "Choose an item from the results." : "",
    );
  }, [required, value]);
  useEffect(() => {
    setOpen(false);
    setQuery("");
    setPage(1);
    setOptions(initial);
    setSelected(initial.find((o) => o.value === value));
  }, [path]);
  useEffect(() => {
    if (!value) {
      setSelected(undefined);
      return;
    }
    const known =
      initial.find((o) => o.value === value) ||
      options.find((o) => o.value === value);
    if (known) {
      setSelected(known);
      return;
    }
    let live = true;
    api<Page>(
      path +
        (path.includes("?") ? "&" : "?") +
        "limit=50&q=" +
        encodeURIComponent(value),
    )
      .then((v) => {
        if (live) setSelected(convert(v.items).find((o) => o.value === value));
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [value, path]);
  useEffect(() => {
    if (!open) return;
    let live = true;
    setBusy(true);
    const timer = setTimeout(
      () =>
        api<Page>(
          path +
            (path.includes("?") ? "&" : "?") +
            `page=${page}&limit=50&q=${encodeURIComponent(query)}`,
        )
          .then((v) => {
            if (!live) return;
            const next = convert(v.items);
            setOptions((old) =>
              page === 1
                ? next
                : [
                    ...old,
                    ...next.filter(
                      (n) => !old.some((o) => o.value === n.value),
                    ),
                  ],
            );
            setMore(!!v.pagination?.has_more);
            setError("");
          })
          .catch((e) => {
            if (live) setError(e.message);
          })
          .finally(() => {
            if (live) setBusy(false);
          }),
      150,
    );
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [
    path,
    open,
    page,
    query,
    valueKey,
    labelKey,
    excludeInactive,
    excludeControl,
    excludePlatform,
  ]);
  function choose(o: Option) {
    setSelected(o);
    onChange(o.value);
    setOpen(false);
    setQuery("");
  }
  return (
    <span className="search-combobox" ref={root}>
      <span className="combobox-input">
        <input
          ref={input}
          role="combobox"
          aria-label={label}
          aria-expanded={open}
          aria-controls={list.id}
          aria-autocomplete="list"
          aria-activedescendant={list.activeId}
          required={required && !value}
          autoComplete="off"
          value={
            open ? query : selected?.label || (value ? "Selected item" : "")
          }
          placeholder={`Choose ${label.toLowerCase()}…`}
          onFocus={() => {
            setOpen(true);
            setQuery("");
            setPage(1);
          }}
          onChange={(e) => {
            setOpen(true);
            setQuery(e.target.value);
            setOptions([]);
            setMore(false);
            setBusy(true);
            setPage(1);
            setActive(0);
            if (value) onChange("");
          }}
          onBlur={(e) => {
            if (!root.current?.contains(e.relatedTarget)) setOpen(false);
          }}
          onKeyDown={list.onKeyDown}
        />
        <ChevronDown size={15} />
      </span>
      {open && (
        <ListboxPopup
          list={list}
          label={label}
          options={options}
          value={value}
          choose={choose}
        >
          {busy && <small>Searching…</small>}
          {!busy && !options.length && !error && <small>No matches</small>}
          {error && <small role="alert">{error}</small>}
          {more && (
            <button
              type="button"
              className="text-button"
              disabled={busy}
              onClick={() => setPage((p) => p + 1)}
            >
              Load more
            </button>
          )}
        </ListboxPopup>
      )}
    </span>
  );
}
