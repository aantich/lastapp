import type { Key, ReactNode } from "react";
import { cx } from "./utils";

export type RecordRow = {
  id: Key;
  // Usually an Avatar.
  leading?: ReactNode;
  title: ReactNode;
  // One muted line under the title (Space, owner, scope).
  meta?: ReactNode;
  // Secondary state shown from 800 px up (a deployment badge).
  detail?: ReactNode;
  // Always shown at the end: a state badge, a time, an Open button.
  trailing?: ReactNode;
};

// A compact list of records inside a card (Home's recent Apps and connected
// agents): a leading avatar, the name with one meta line, and trailing
// states or actions. Names and meta lines truncate rather than wrap; for a
// list where one record opens beside it use PickList, for many columns
// DataTable.
export function RecordList({
  label,
  rows,
  className,
}: {
  label: string;
  rows: RecordRow[];
  className?: string;
}) {
  return (
    <ul className={cx("ui-record-list", className)} aria-label={label}>
      {rows.map((row) => (
        <li key={row.id}>
          {row.leading}
          <span className="ui-record-copy">
            <strong>{row.title}</strong>
            {row.meta && <small>{row.meta}</small>}
          </span>
          {row.detail && <span className="ui-record-detail">{row.detail}</span>}
          {row.trailing && (
            <span className="ui-record-trailing">{row.trailing}</span>
          )}
        </li>
      ))}
    </ul>
  );
}
