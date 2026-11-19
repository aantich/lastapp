import { useId, type CSSProperties, type Key, type ReactNode } from "react";
import { cx } from "./utils";

export type RowTableRow = {
  id: Key;
  // One node per column, in the order of `columns`.
  cells: ReactNode[];
  // Makes the whole row open something: the first cell becomes its button
  // (its content is the button's name) and the row is the click target.
  onOpen?: () => void;
};

// Cell content: a title over a muted meta line, a muted mono value (an
// address, an order, a reference) and right-aligned row actions.
export function CellTitle({
  title,
  meta,
}: {
  title: ReactNode;
  meta?: ReactNode;
}) {
  return (
    <span className="ui-row-meta">
      <span>{title}</span>
      {meta && <span>{meta}</span>}
    </span>
  );
}
export function CellMono({ children }: { children: ReactNode }) {
  return <span className="ui-row-mono">{children}</span>;
}
export function CellActions({ children }: { children: ReactNode }) {
  return <span className="ui-row-actions">{children}</span>;
}

// A card of grid rows with fixed column tracks (Apps, an App's releases and
// micro-apps): an optional title line, an optional column-header row, then
// one row per record. Rows keep table semantics (role=table/row/cell); a
// row with `onOpen` is opened by its first cell's button, stretched over the
// row. Narrow screens scroll the rows sideways inside the card.
export function RowTable({
  label,
  columns,
  headers,
  rows,
  title,
  note,
  actions,
  empty,
  footer,
  className,
  rowClassName,
}: {
  // Names the table when there is no title.
  label: string;
  // A CSS grid-template-columns value shared by every row.
  columns: string;
  headers?: ReactNode[];
  rows: RowTableRow[];
  title?: ReactNode;
  // Muted text beside the title.
  note?: ReactNode;
  actions?: ReactNode;
  // Shown instead of rows when there are none.
  empty?: ReactNode;
  footer?: ReactNode;
  className?: string;
  // An e2e hook on every record row.
  rowClassName?: string;
}) {
  const heading = useId();
  return (
    <section
      className={cx("panel ui-row-table", className)}
      aria-labelledby={title ? heading : undefined}
    >
      {title && (
        <header className="ui-row-table-head">
          <h2 id={heading}>{title}</h2>
          {note && <span>{note}</span>}
          {actions && <div className="ui-row-table-actions">{actions}</div>}
        </header>
      )}
      {rows.length || headers ? (
        <div className="ui-row-table-scroll">
          <div
            role="table"
            aria-label={title ? undefined : label}
            aria-labelledby={title ? heading : undefined}
            style={{ "--ui-row-columns": columns } as CSSProperties}
          >
            {headers && (
              <div role="row" className="ui-row ui-row-head">
                {headers.map((h, i) => (
                  <span role="columnheader" key={i}>
                    {h}
                  </span>
                ))}
              </div>
            )}
            {rows.map((row) => (
              <div
                role="row"
                key={row.id}
                className={cx(
                  "ui-row",
                  row.onOpen && "ui-row-link",
                  rowClassName,
                )}
              >
                {row.cells.map((cell, i) => (
                  <span role="cell" key={i}>
                    {i === 0 && row.onOpen ? (
                      <button
                        type="button"
                        className="ui-row-open"
                        onClick={row.onOpen}
                      >
                        {cell}
                      </button>
                    ) : (
                      cell
                    )}
                  </span>
                ))}
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {!rows.length && empty && (
        <div className="ui-row-table-empty">{empty}</div>
      )}
      {footer && <footer className="ui-row-table-foot">{footer}</footer>}
    </section>
  );
}
