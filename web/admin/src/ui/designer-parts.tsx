import { useId, type ReactNode } from "react";
import { cx } from "./utils";
export { NavList, NavListNote, type NavListItem } from "./nav-list";
export { GridTable, type GridColumn, type GridRow } from "./grid-table";

// Layout parts of a record designer (the entity designer): the record's
// header, the list of records beside it, a definition grid, titled panels,
// access paths, a dashed call to action, a row of chips and result tiles.

// The open record's header: an initial tile, the name as the page's h1, a
// mono meta line, state chips and the record's actions on the right.
export function RecordHeader({
  initial,
  title,
  meta,
  chips,
  actions,
}: {
  initial: string;
  title: ReactNode;
  meta?: ReactNode;
  chips?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="ui-record-header">
      <span className="ui-record-tile" aria-hidden>
        {initial}
      </span>
      <div className="ui-record-title">
        <h1>{title}</h1>
        {meta && <span>{meta}</span>}
      </div>
      {chips}
      {actions && <div className="ui-record-actions">{actions}</div>}
    </div>
  );
}

// A titled card: an h2 with an optional muted lead and trailing actions,
// then its content in a column. `flush` drops the padding for a GridTable.
export function Panel({
  title,
  lead,
  actions,
  flush,
  children,
}: {
  title: ReactNode;
  lead?: ReactNode;
  actions?: ReactNode;
  flush?: boolean;
  children?: ReactNode;
}) {
  const id = useId();
  return (
    <section
      className={cx("ui-panel panel", flush && "ui-panel-flush")}
      aria-labelledby={id}
    >
      <header className="ui-panel-head">
        <h2 id={id}>{title}</h2>
        {lead && <span>{lead}</span>}
        {actions && <div className="ui-panel-actions">{actions}</div>}
      </header>
      {children}
    </section>
  );
}

// One access path: who (a chip), "can", the operations, the record
// condition and the fields, kept together in one box.
export function PathCard({
  who,
  whoTone = "inverse",
  operations,
  conditions = [],
  fields,
  aside,
}: {
  who: ReactNode;
  whoTone?: "inverse" | "success" | "neutral";
  operations: ReactNode[];
  conditions?: ReactNode[];
  fields?: ReactNode;
  // Trailing content of the first line (the path's name and actions).
  aside?: ReactNode;
}) {
  return (
    <div className="ui-path">
      <div className="ui-path-line">
        <span className={cx("ui-tag ui-pill", "tone-" + whoTone)}>{who}</span>
        <span className="ui-path-can">can</span>
        {operations.map((o, i) => (
          <span className="ui-tag ui-pill" key={i}>
            {o}
          </span>
        ))}
        {aside && <span className="ui-path-aside">{aside}</span>}
      </div>
      {conditions.map((c, i) => (
        <div className="ui-path-condition" key={i}>
          {c}
        </div>
      ))}
      {fields && <span className="ui-path-fields">{fields}</span>}
    </div>
  );
}

// A dashed call to action inside a panel: a title, a sentence and a button.
export function Callout({
  title,
  children,
  action,
}: {
  title: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="ui-callout">
      <span className="ui-callout-copy">
        <strong>{title}</strong>
        {children && <span>{children}</span>}
      </span>
      {action}
    </div>
  );
}

// A boxed row: a kind chip, a mono expression and a muted note on the
// right, then optional actions.
export function ChipRow({
  chip,
  expression,
  note,
  actions,
}: {
  chip: ReactNode;
  expression: ReactNode;
  note?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="ui-chip-row">
      <span className="ui-tag ui-pill">{chip}</span>
      <span className="ui-chip-row-expression">{expression}</span>
      {note && <span className="ui-chip-row-note">{note}</span>}
      {actions}
    </div>
  );
}

// A tinted result tile: a small label over a large value (`size="lg"`) or
// a mono value.
export function ResultTile({
  label,
  children,
  size = "lg",
  grow = 1,
}: {
  label: ReactNode;
  children: ReactNode;
  size?: "lg" | "mono";
  grow?: 1 | 2;
}) {
  return (
    <div className={cx("ui-result-tile", grow === 2 && "ui-result-wide")}>
      <span>{label}</span>
      <div className={size === "lg" ? "ui-result-value" : "ui-result-mono"}>
        {children}
      </div>
    </div>
  );
}

export function ResultTiles({ children }: { children: ReactNode }) {
  return <div className="ui-result-tiles">{children}</div>;
}

// The designer page: header, tabs and body one above the other (24 px).
export function DesignerPage({ children }: { children: ReactNode }) {
  return <div className="ui-designer">{children}</div>;
}

// The list of records beside the open record's content; the list wraps
// above the content on narrow screens.
export function DesignerBody({
  nav,
  children,
}: {
  nav: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="ui-designer-body">
      {nav}
      <div className="ui-designer-main">{children}</div>
    </div>
  );
}

// A sentence under a result (13 px, neutral).
export function DesignerNote({ children }: { children: ReactNode }) {
  return <p className="ui-designer-note">{children}</p>;
}
