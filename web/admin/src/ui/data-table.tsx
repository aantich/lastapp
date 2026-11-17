import type { Key, ReactNode } from "react";
import { cx } from "./utils";
import { Skeleton } from "./feedback";
import { EmptyState } from "./empty-state";

export type Column<Row> = {
  // Stable key; also the React key of the cell.
  key: string;
  header: ReactNode;
  render: (row: Row) => ReactNode;
  // Numbers align right with tabular figures.
  numeric?: boolean;
  className?: string;
};

// A table of records: the console's table markup (.table-wrap/table) with a
// header row, optional row actions, loading skeleton rows and an empty
// state. Data fetching, search and paging stay with the caller (Collection
// adds the shared server search/paging on top).
export function DataTable<Row>({
  columns,
  rows,
  rowKey,
  actions,
  caption,
  loading,
  empty = "No records yet.",
  skeletonRows = 3,
}: {
  columns: Column<Row>[];
  rows: Row[];
  rowKey: (row: Row) => Key | undefined;
  actions?: (row: Row) => ReactNode;
  // Visually hidden name of the table for screen readers.
  caption?: string;
  loading?: boolean;
  empty?: ReactNode;
  skeletonRows?: number;
}) {
  return (
    <>
      <div className="table-wrap">
        <table className="ui-table" aria-busy={loading || undefined}>
          {caption && <caption className="ui-sr-only">{caption}</caption>}
          <thead>
            <tr>
              {columns.map((c) => (
                <th
                  key={c.key}
                  scope="col"
                  className={cx(c.numeric && "ui-num", c.className)}
                >
                  {c.header}
                </th>
              ))}
              {actions && (
                <th scope="col" className="actions-column">
                  Actions
                </th>
              )}
            </tr>
          </thead>
          <tbody>
            {loading
              ? Array.from({ length: skeletonRows }, (_, i) => (
                  <tr key={i}>
                    {columns.map((c) => (
                      <td key={c.key}>
                        <Skeleton width={i % 2 ? "60%" : "80%"} />
                      </td>
                    ))}
                    {actions && <td className="actions-column" />}
                  </tr>
                ))
              : rows.map((row) => (
                  <tr key={rowKey(row)}>
                    {columns.map((c) => (
                      <td
                        key={c.key}
                        className={cx(c.numeric && "ui-num", c.className)}
                      >
                        {c.render(row)}
                      </td>
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
      {!loading && !rows.length && <EmptyState>{empty}</EmptyState>}
    </>
  );
}
