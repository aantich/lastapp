import { useEffect, useState } from "react";
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
  const [query, setQuery] = useState(""),
    [page, setPage] = useState(1),
    [options, setOptions] = useState<Option[]>(initial);
  const [selected, setSelected] = useState<Option | undefined>(
    initial.find((o) => o.value === value),
  );
  const [more, setMore] = useState(false),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let active = true;
    const timer = setTimeout(() => {
      setBusy(true);
      api(
        path +
          `${path.includes("?") ? "&" : "?"}page=${page}&limit=50&q=${encodeURIComponent(query)}`,
      )
        .then((v) => {
          if (!active) return;
          setOptions(
            v.items
              .filter(
                (x: Record<string, any>) =>
                  (!excludeControl || x.domain !== "control") &&
                  (!excludePlatform || !x.platform_admin) &&
                  (!excludeInactive ||
                    ((!x.state || x.state === "active") &&
                      (!x.user_state || x.user_state === "active") &&
                      (!x.tenant_state || x.tenant_state === "active"))),
              )
              .map((x: Record<string, any>) => ({
                value: x[valueKey],
                label: x[labelKey],
              })),
          );
          setMore(!!v.pagination?.has_more);
          setError("");
        })
        .catch((e) => {
          if (active) setError(e.message);
        })
        .finally(() => {
          if (active) setBusy(false);
        });
    }, 200);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [
    path,
    page,
    query,
    valueKey,
    labelKey,
    excludeInactive,
    excludeControl,
    excludePlatform,
  ]);
  return (
    <span className="lookup-select">
      <input
        aria-label={`Search ${label}`}
        placeholder={`Search ${label.toLowerCase()}…`}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setPage(1);
        }}
      />
      <select
        aria-label={label}
        value={value}
        required={required}
        onChange={(e) => {
          setSelected(options.find((o) => o.value === e.target.value));
          onChange(e.target.value);
        }}
      >
        <option value="">Choose…</option>
        {value && !options.some((o) => o.value === value) && (
          <option value={value}>{selected?.label || value}</option>
        )}
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      <span className="lookup-controls">
        <button
          type="button"
          disabled={busy || page === 1}
          onClick={() => setPage((p) => p - 1)}
          aria-label={`Previous ${label} options`}
        >
          Previous
        </button>
        <small>Page {page}</small>
        <button
          type="button"
          disabled={busy || !more}
          onClick={() => setPage((p) => p + 1)}
          aria-label={`Next ${label} options`}
        >
          Next
        </button>
      </span>
      {error && <small role="alert">{error}</small>}
    </span>
  );
}
