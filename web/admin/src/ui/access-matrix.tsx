import type { ReactNode } from "react";
import { Check } from "lucide-react";
import { cx } from "./utils";

export type AccessMatrixRow = {
  id: string;
  name: ReactNode;
  // Plain text for the cells' accessible names.
  label: string;
  // Muted line under the name: who holds it, a record condition.
  detail?: ReactNode;
};
// A cell's state: granted or not, and whether the unpublished draft
// changes it ("added" / "removed").
export type AccessCell = { on: boolean; draft?: "added" | "removed" };

// Who may do what to one entity, read-only, as a card: a title line (title,
// a muted note, actions at the end), then rows (roles or relationships) ×
// operation columns, each cell a mark. With `onOpen` a cell is a button
// that opens where the permission is edited (a permission path is an
// operation with fields and a condition, so a single cell is never toggled
// here). A draft change shows as a dashed outline around the published
// state. `children` replaces the table (loading, empty).
export function AccessMatrix({
  title,
  note,
  actions,
  label,
  rowHeader = "Role",
  columns,
  rows,
  cell,
  onOpen,
  children,
}: {
  title: ReactNode;
  note?: ReactNode;
  actions?: ReactNode;
  label: string;
  rowHeader?: string;
  columns: { id: string; label: string }[];
  rows: AccessMatrixRow[];
  cell: (row: string, column: string) => AccessCell;
  onOpen?: (row: string, column: string) => void;
  children?: ReactNode;
}) {
  return (
    <section className="panel ui-access-card">
      <div className="ui-access-head">
        <h2>{title}</h2>
        {note && <span>{note}</span>}
        {actions && <div className="ui-access-actions">{actions}</div>}
      </div>
      {children ?? (
        <div
          className="ui-access-matrix"
          role="region"
          aria-label={label}
          tabIndex={0}
        >
          <table>
            <caption className="ui-sr-only">{label}</caption>
            <thead>
              <tr>
                <th scope="col">{rowHeader}</th>
                {columns.map((c) => (
                  <th key={c.id} scope="col">
                    {c.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.id}>
                  <th scope="row">
                    <strong>{r.name}</strong>
                    {r.detail && <span>{r.detail}</span>}
                  </th>
                  {columns.map((c) => {
                    const v = cell(r.id, c.id);
                    const state =
                      (v.on ? "allowed" : "not allowed") +
                      (v.draft === "added"
                        ? "; the draft allows it"
                        : v.draft === "removed"
                          ? "; the draft removes it"
                          : "");
                    const mark = (
                      <span
                        className={cx(
                          "ui-access-mark",
                          v.on && "is-on",
                          v.draft && "is-draft",
                        )}
                        aria-hidden
                      >
                        {v.on && <Check size={12} strokeWidth={3} />}
                      </span>
                    );
                    return (
                      <td key={c.id}>
                        {onOpen ? (
                          <button
                            type="button"
                            aria-label={`${r.label} ${c.label}: ${state}. Open the permission`}
                            onClick={() => onOpen(r.id, c.id)}
                          >
                            {mark}
                          </button>
                        ) : (
                          <span
                            role="img"
                            aria-label={`${r.label} ${c.label}: ${state}`}
                          >
                            {mark}
                          </span>
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

export type Assignment = {
  id: string;
  role: ReactNode;
  who: ReactNode;
  scope?: ReactNode;
  action?: ReactNode;
};

// Role assignments as a card of lines: the role (an ink chip) → who holds
// it, the scope chip at the end and an optional action. `children` replaces
// the list (loading, empty); `footer` follows it (paging).
export function AssignmentList({
  title,
  actions,
  label,
  items,
  children,
  footer,
}: {
  title: ReactNode;
  actions?: ReactNode;
  label: string;
  items: Assignment[];
  children?: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <section className="panel ui-assignments-card">
      <div className="ui-access-head">
        <h2>{title}</h2>
        {actions && <div className="ui-access-actions">{actions}</div>}
      </div>
      {children ?? (
        <ul className="ui-assignments" aria-label={label}>
          {items.map((a) => (
            <li key={a.id}>
              <span className="ui-tag ui-pill tone-inverse">{a.role}</span>
              <span className="ui-assignment-arrow" aria-hidden>
                →
              </span>
              <strong>{a.who}</strong>
              {a.scope && <span className="ui-tag ui-pill">{a.scope}</span>}
              {a.action}
            </li>
          ))}
        </ul>
      )}
      {footer}
    </section>
  );
}
