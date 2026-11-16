import { useEffect, useState, type ReactNode } from "react";
import { FolderOpen, Search } from "lucide-react";
import { api } from "./api";
import type { Item, Page } from "./api-types";
// All Tenant collection screens share server search/paging and scope-filtered APIs.
type CollectionProps = {
  path: string;
  reload: number;
  columns: [string, (row: Item) => ReactNode][];
  actions?: (row: Item) => ReactNode;
  empty?: string;
};
export function Collection(props: CollectionProps) {
  return <ScopedCollection key={props.path} {...props} />;
}
function ScopedCollection({
  path,
  reload,
  columns,
  actions,
  empty = "No records yet.",
}: CollectionProps) {
  const [items, setItems] = useState<Item[]>([]),
    [query, setQuery] = useState(""),
    [page, setPage] = useState(1),
    [more, setMore] = useState(false),
    [loading, setLoading] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    setLoading(true);
    setError("");
    const timer = setTimeout(() => {
      api<Page>(
        path +
          (path.includes("?") ? "&" : "?") +
          `page=${page}&q=${encodeURIComponent(query)}`,
      )
        .then((v) => {
          if (live) {
            setItems(v.items);
            setMore(!!v.pagination?.has_more);
          }
        })
        .catch((e) => {
          if (live) setError(e.message);
        })
        .finally(() => {
          if (live) setLoading(false);
        });
    }, 150);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [path, page, query, reload]);
  return (
    <section className="panel list-panel">
      <div className="toolbar">
        <label className="search">
          <Search size={16} />
          <input
            aria-label="Search records"
            placeholder="Search records…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setItems([]);
              setMore(false);
              setPage(1);
            }}
          />
        </label>
        <span>{loading ? "Loading…" : `${items.length} records`}</span>
      </div>
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {columns.map(([label]) => (
                <th key={label}>{label}</th>
              ))}
              {actions && <th className="actions-column">Actions</th>}
            </tr>
          </thead>
          <tbody>
            {items.map((row) => (
              <tr key={row.id || row.user_id}>
                {columns.map(([label, render]) => (
                  <td key={label}>{render(row)}</td>
                ))}
                {actions && (
                  <td className="actions-column">
                    <div className="row-actions">{actions(row)}</div>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {!loading && !items.length && (
        <div className="empty">
          <FolderOpen size={28} />
          <p>{empty}</p>
        </div>
      )}
      {(page > 1 || more) && (
        <div className="pagination">
          <button
            className="secondary"
            aria-label="Previous records"
            disabled={loading || page === 1}
            onClick={() => setPage((p) => p - 1)}
          >
            Previous
          </button>
          <span>Page {page}</span>
          <button
            className="secondary"
            aria-label="Next records"
            disabled={loading || !more}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </button>
        </div>
      )}
    </section>
  );
}
