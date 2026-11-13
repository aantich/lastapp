import { useEffect, useId, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { api } from "./api";
type Option = { value: string; label: string };
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
  const id = useId(),
    root = useRef<HTMLSpanElement>(null),
    input = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false),
    [query, setQuery] = useState(""),
    [page, setPage] = useState(1),
    [options, setOptions] = useState<Option[]>(initial),
    [selected, setSelected] = useState<Option | undefined>(
      initial.find((o) => o.value === value),
    ),
    [more, setMore] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [active, setActive] = useState(0);
  const convert = (items: Record<string, any>[]) =>
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
    api(
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
        api(
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
  useEffect(() => {
    const close = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, []);
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
          aria-controls={id}
          aria-autocomplete="list"
          aria-activedescendant={
            open && options[active] ? id + "-" + active : undefined
          }
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
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" || e.key === "ArrowUp") {
              e.preventDefault();
              setOpen(true);
              setActive((i) =>
                Math.max(
                  0,
                  Math.min(
                    options.length - 1,
                    i + (e.key === "ArrowDown" ? 1 : -1),
                  ),
                ),
              );
            }
            if (e.key === "Enter" && open) {
              e.preventDefault();
              if (options[active]) choose(options[active]);
            }
            if (e.key === "Escape" && open) {
              e.preventDefault();
              e.stopPropagation();
              setOpen(false);
            }
            if (e.key === "Tab") setOpen(false);
          }}
        />
        <ChevronDown size={15} />
      </span>
      {open && (
        <span className="combobox-popup">
          <span role="listbox" id={id} aria-label={label}>
            {options.map((o, i) => (
              <button
                type="button"
                role="option"
                id={id + "-" + i}
                key={o.value}
                aria-selected={value === o.value}
                className={active === i ? "focused" : ""}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => choose(o)}
                onMouseMove={() => setActive(i)}
              >
                {o.label}
              </button>
            ))}
          </span>
          {busy && <small>Searching…</small>}
          {!busy && !options.length && !error && <small>No matches</small>}
          {error && <small role="alert">{error}</small>}
          {more && (
            <button
              type="button"
              className="text-button"
              disabled={busy}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => setPage((p) => p + 1)}
            >
              Load more
            </button>
          )}
        </span>
      )}
    </span>
  );
}
