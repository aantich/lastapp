import { useEffect, useState, type ReactNode } from "react";
import { Search } from "lucide-react";
import { Alert } from "./alert";
import { Button } from "./button";
import { DataTable } from "./data-table";
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
      {error && <Alert>{error}</Alert>}
      <DataTable
        columns={columns.map(([label, render]) => ({
          key: label,
          header: label,
          render,
        }))}
        rows={items}
        rowKey={(row) => keyOf(row.id || row.user_id)}
        actions={actions}
        loading={loading}
        empty={empty}
      />
      {(page > 1 || more) && (
        <div className="pagination">
          <Button
            aria-label="Previous records"
            disabled={loading || page === 1}
            onClick={() => setPage((p) => p - 1)}
          >
            Previous
          </Button>
          <span>Page {page}</span>
          <Button
            aria-label="Next records"
            disabled={loading || !more}
            onClick={() => setPage((p) => p + 1)}
          >
            Next
          </Button>
        </div>
      )}
    </section>
  );
}
