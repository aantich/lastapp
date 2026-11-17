import { useEffect, useState, type ReactNode } from "react";
import { FolderOpen, Search } from "lucide-react";
import { keyOf, type Item, type Page } from "../api-types";
import { useApiQuery } from "../query";
// All Tenant collection screens share server search/paging and scope-filtered APIs.
type CollectionProps = {
  path: string;
  columns: [string, (row: Item) => ReactNode][];
  actions?: (row: Item) => ReactNode;
  empty?: string;
};
export function Collection(props: CollectionProps) {
  return <ScopedCollection key={props.path} {...props} />;
}
function ScopedCollection({
  path,
  columns,
  actions,
  empty = "No records yet.",
}: CollectionProps) {
  const [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [page, setPage] = useState(1);
  // Typing waits briefly before searching so each keystroke is not a request.
  useEffect(() => {
    const timer = setTimeout(() => setSearch(query), 150);
    return () => clearTimeout(timer);
  }, [query]);
  const result = useApiQuery<Page>(
      path +
        (path.includes("?") ? "&" : "?") +
        `page=${page}&q=${encodeURIComponent(search)}`,
    ),
    loading = result.isPending || query !== search,
    items = (!loading && result.data?.items) || [],
    more = !loading && !!result.data?.pagination?.has_more,
    error = result.error?.message || "";
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
              <tr key={keyOf(row.id || row.user_id)}>
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
