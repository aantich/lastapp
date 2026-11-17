import { useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { api } from "./api";
import { text, type Item, type Page } from "./api-types";
import { useListbox, type ListOption as Option } from "./listbox";
import { apiKey, useApiQuery } from "./query";
import { ListboxPopup } from "./select";

type RemoteSelectProps = {
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
};

// A searchable, paged combobox over a list API. A different path is a
// different list, so it starts with a fresh search and selection.
export function RemoteSelect(props: RemoteSelectProps) {
  return <PathSelect key={props.path} {...props} />;
}

function PathSelect({
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
}: RemoteSelectProps) {
  const root = useRef<HTMLSpanElement>(null),
    input = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [chosen, setChosen] = useState<Option>();
  const url = (params: string) =>
    path + (path.includes("?") ? "&" : "?") + params;
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
        value: text(x[valueKey]),
        label:
          String(x[labelKey] || "") +
          (x.email && labelKey === "display_name"
            ? ` (${String(x.email)})`
            : ""),
      }));
  // Typing waits briefly before searching so each keystroke is not a request.
  useEffect(() => {
    const timer = setTimeout(() => setSearch(query), 150);
    return () => clearTimeout(timer);
  }, [query]);
  const openState = useState(false),
    [open, setOpen] = openState;
  // One cached result per search; "Load more" appends the next page. The
  // AbortSignal cancels a superseded or abandoned request.
  const results = useInfiniteQuery({
    queryKey: [...apiKey(path), "search", search],
    queryFn: ({ pageParam, signal }) =>
      api<Page>(
        url(`page=${pageParam}&limit=50&q=${encodeURIComponent(search)}`),
        "GET",
        undefined,
        undefined,
        signal,
      ),
    initialPageParam: 1,
    getNextPageParam: (last, _pages, page) =>
      last.pagination?.has_more ? page + 1 : undefined,
    enabled: open,
  });
  const typing = query !== search;
  const options: Option[] = typing
    ? []
    : results.data
      ? results.data.pages
          .flatMap((p) => convert(p.items))
          .filter(
            (o, i, all) => all.findIndex((x) => x.value === o.value) === i,
          )
      : search
        ? []
        : initial;
  const busy = typing || results.isPending || results.isFetchingNextPage,
    more = !typing && results.hasNextPage,
    error = results.error?.message || "";
  // A value set from outside that is not among the loaded options is looked
  // up by search so the input can show its label.
  const known =
    (chosen?.value === value ? chosen : undefined) ||
    initial.find((o) => o.value === value) ||
    options.find((o) => o.value === value);
  const lookup = useApiQuery<Page>(
    value && !known ? url("limit=50&q=" + encodeURIComponent(value)) : null,
  );
  const selected = value
    ? known ||
      (lookup.data && convert(lookup.data.items).find((o) => o.value === value))
    : undefined;
  const list = useListbox(options, choose, root, openState),
    { setActive } = list;
  useEffect(() => {
    input.current?.setCustomValidity(
      required && !value ? "Choose an item from the results." : "",
    );
  }, [required, value]);
  function choose(o: Option) {
    setChosen(o);
    onChange(o.value);
    setOpen(false);
    setQuery("");
    setSearch("");
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
            setSearch("");
          }}
          onChange={(e) => {
            setOpen(true);
            setQuery(e.target.value);
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
              onClick={() => void results.fetchNextPage()}
            >
              Load more
            </button>
          )}
        </ListboxPopup>
      )}
    </span>
  );
}
