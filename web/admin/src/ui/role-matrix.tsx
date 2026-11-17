import type { ReactNode } from "react";
import { Checkbox } from "./checkbox";

export type MatrixRow = { id: string; name: string; description?: ReactNode };
export type MatrixColumn = { id: string; label: string };

// Roles × permissions as a table of checkboxes. Each cell is named
// "<permission> for <role>"; a column header checkbox sets the permission
// for every role and shows "mixed" when only some roles have it. `value`
// maps a role id to its granted permission ids.
export function RoleMatrix({
  label,
  rows,
  columns,
  value,
  onChange,
  disabled,
  rowHeader = "Role",
}: {
  label: string;
  rows: MatrixRow[];
  columns: MatrixColumn[];
  value: Record<string, string[]>;
  onChange: (next: Record<string, string[]>) => void;
  disabled?: boolean;
  rowHeader?: string;
}) {
  const has = (row: string, column: string) =>
    (value[row] || []).includes(column);
  function set(row: string, column: string, on: boolean) {
    const current = value[row] || [];
    onChange({
      ...value,
      [row]: on
        ? columns
            .map((c) => c.id)
            .filter((id) => id === column || current.includes(id))
        : current.filter((id) => id !== column),
    });
  }
  function setColumn(column: string, on: boolean) {
    const next = { ...value };
    for (const row of rows) {
      const current = next[row.id] || [];
      next[row.id] = on
        ? columns
            .map((c) => c.id)
            .filter((id) => id === column || current.includes(id))
        : current.filter((id) => id !== column);
    }
    onChange(next);
  }
  return (
    <div className="table-wrap" role="region" aria-label={label} tabIndex={0}>
      <table className="ui-table ui-matrix">
        <caption className="ui-sr-only">{label}</caption>
        <thead>
          <tr>
            <th scope="col">{rowHeader}</th>
            {columns.map((c) => {
              const count = rows.filter((r) => has(r.id, c.id)).length;
              return (
                <th key={c.id} scope="col">
                  <span>
                    {c.label}
                    <Checkbox
                      aria-label={`${c.label} for every ${rowHeader.toLowerCase()}`}
                      checked={rows.length > 0 && count === rows.length}
                      indeterminate={count > 0 && count < rows.length}
                      disabled={disabled || !rows.length}
                      onChange={(on) => setColumn(c.id, on)}
                    />
                  </span>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <th scope="row">
                <strong>{r.name}</strong>
                {r.description && <small>{r.description}</small>}
              </th>
              {columns.map((c) => (
                <td key={c.id}>
                  <Checkbox
                    aria-label={`${c.label} for ${r.name}`}
                    checked={has(r.id, c.id)}
                    disabled={disabled}
                    onChange={(on) => set(r.id, c.id, on)}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
