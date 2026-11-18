import { useEffect, useState, type ReactNode } from "react";
import { Search } from "lucide-react";
import { Alert } from "./alert";
import { Button } from "./button";
import { DataTable } from "./data-table";
import { EmptyState } from "./empty-state";
import { Skeleton } from "./feedback";
import { keyOf, type Item, type Page } from "../api-types";
import { useApiQuery } from "../query";
// All Tenant collection screens share server search/paging and scope-filtered APIs.
type CollectionProps = {
  path: string;
  columns: [string, (row: Item) => ReactNode][];
  actions?: (row: Item) => ReactNode;
  empty?: ReactNode;
  // Names the table (default "Records").
  caption?: string;
  // Server filters beside the search box: their query string (without
  // "?"/"&") restarts paging; `filters` renders their controls.
  params?: string;
  filters?: ReactNode;
  // Renders the page's rows instead of a table (e.g. a Timeline feed).
  render?: (rows: Item[]) => ReactNode;
  // Refetch interval in ms while `poll(rows)` says the page is changing
  // (e.g. a workspace is still preparing); false stops.
  poll?: (rows: Item[]) => number | false;
};
export function Collection(props: CollectionProps) {
  return (
    <ScopedCollection
      key={props.path + "?" + (props.params || "")}
      {...props}
    />
  );
}
function ScopedCollection({
  path,
  columns,
  actions,
  empty = "No records yet.",
  caption = "Records",
  params,
  filters,
  render,
  poll,
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
        `page=${page}&q=${encodeURIComponent(search)}` +
        (params ? "&" + params : ""),
      poll
        ? { refetchInterval: (q) => poll(q.state.data?.items || []) }
        : undefined,
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
        {filters && <div className="ui-collection-filters">{filters}</div>}
        <span>{loading ? "Loading…" : `${items.length} records`}</span>
      </div>
      {error && <Alert>{error}</Alert>}
      {render ? (
        loading ? (
          <div className="ui-collection-feed" aria-busy>
            <Skeleton width="70%" />
            <Skeleton width="55%" />
            <Skeleton width="62%" />
          </div>
        ) : items.length ? (
          <div className="ui-collection-feed">{render(items)}</div>
        ) : (
          <EmptyState>{empty}</EmptyState>
        )
      ) : (
        <DataTable
          caption={caption}
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
      )}
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
