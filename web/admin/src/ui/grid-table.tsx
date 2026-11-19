import type { CSSProperties, ReactNode } from "react";

export type GridColumn = {
  key: string;
  header: ReactNode;
  // A CSS grid track, e.g. "minmax(150px,1.4fr)".
  track: string;
  // Muted cell text (notes); the header is only spoken (an actions column).
  muted?: boolean;
  hideHeader?: boolean;
};

export type GridRow = {
  key: string;
  cells: Record<string, ReactNode>;
  // Marks a row as new: a tinted background and an accent bar.
  highlight?: boolean;
};

// A definition grid (fields and their behaviour): a header row and rows on
// one grid template, scrolling sideways inside its card when narrow. It is
// a table for assistive technology.
export function GridTable({
  label,
  columns,
  rows,
}: {
  label: string;
  columns: GridColumn[];
  rows: GridRow[];
}) {
  const style = {
    "--ui-grid-tracks": columns.map((c) => c.track).join(" "),
  } as CSSProperties;
  return (
    <div className="ui-grid-table-scroll">
      <div
        className="ui-grid-table"
        role="table"
        aria-label={label}
        style={style}
      >
        <div role="rowgroup">
          <div role="row" className="ui-grid-head">
            {columns.map((c) => (
              <span role="columnheader" key={c.key}>
                {c.hideHeader ? (
                  <span className="ui-sr-only">{c.header}</span>
                ) : (
                  c.header
                )}
              </span>
            ))}
          </div>
        </div>
        <div role="rowgroup">
          {rows.map((row) => (
            <div
              role="row"
              key={row.key}
              data-highlight={row.highlight || undefined}
            >
              {columns.map((c) => (
                <span role="cell" key={c.key} data-muted={c.muted || undefined}>
                  {row.cells[c.key]}
                </span>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
