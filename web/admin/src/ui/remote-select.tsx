import { useEffect, useRef, useState } from "react";
import { ChevronDown } from "lucide-react";
import { text, type Item, type Page } from "../api-types";
import { useListbox, type ListOption as Option } from "./listbox";
import { useApiQuery } from "../query";
import { ListboxPopup } from "./select";
import {
  useRemoteOptions,
  withParams,
  type RemoteOptions,
} from "./remote-options";

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
  // Wiring from a surrounding FormField (its label, hint and error).
  id?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
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
  id,
  "aria-describedby": describedBy,
  "aria-invalid": invalid,
}: RemoteSelectProps) {
  const root = useRef<HTMLSpanElement>(null),
    input = useRef<HTMLInputElement>(null);
  const [chosen, setChosen] = useState<Option>();
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
  const openState = useState(false),
    [open, setOpen] = openState;
  const remote = useRemoteOptions({ path, open, convert, initial }),
    { options } = remote;
  // A value set from outside that is not among the loaded options is looked
  // up by search so the input can show its label.
  const known =
    (chosen?.value === value ? chosen : undefined) ||
    initial.find((o) => o.value === value) ||
    options.find((o) => o.value === value);
  const lookup = useApiQuery<Page>(
    value && !known
      ? withParams(path, "limit=50&q=" + encodeURIComponent(value))
      : null,
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
    remote.reset();
  }
  return (
    <span className="search-combobox" ref={root}>
      <span className="combobox-input">
        <input
          ref={input}
          id={id}
          className="ui-input"
          role="combobox"
          aria-label={label}
          aria-describedby={describedBy}
          aria-invalid={invalid || undefined}
          aria-expanded={open}
          aria-controls={list.id}
          aria-autocomplete="list"
          aria-activedescendant={list.activeId}
          required={required && !value}
          autoComplete="off"
          value={
            open
              ? remote.query
              : selected?.label || (value ? "Selected item" : "")
          }
          placeholder={`Choose ${label.toLowerCase()}…`}
          onFocus={() => {
            setOpen(true);
            remote.reset();
          }}
          onChange={(e) => {
            setOpen(true);
            remote.setQuery(e.target.value);
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
          <RemoteListStatus remote={remote} count={options.length} />
        </ListboxPopup>
      )}
    </span>
  );
}

// The paged list's status line under its options: searching, no matches, a
// request error, and the "Load more" button for the next page.
export function RemoteListStatus({
  remote,
  count,
}: {
  remote: Pick<RemoteOptions, "busy" | "more" | "error" | "loadMore">;
  count: number;
}) {
  const { busy, more, error, loadMore } = remote;
  return (
    <>
      {busy && <small>Searching…</small>}
      {!busy && !count && !error && <small>No matches</small>}
      {error && <small role="alert">{error}</small>}
      {more && (
        <button
          type="button"
          className="text-button"
          disabled={busy}
          onClick={loadMore}
        >
          Load more
        </button>
      )}
    </>
  );
}
