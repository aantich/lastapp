import type { ReactNode } from "react";
import { cx } from "./utils";

// Related controls under one name: a native fieldset whose legend names the
// group for assistive technology (a permission path's fields and records, an
// expression and its arguments). `disabled` disables every control inside.
// `framed` draws a bordered box for a repeatable unit (a permission gate);
// `nested` indents a group that belongs to the one around it (an argument of
// an expression).
export function FieldGroup({
  legend,
  description,
  actions,
  framed,
  nested,
  disabled,
  className,
  children,
}: {
  legend: ReactNode;
  description?: ReactNode;
  // Buttons for the whole group (Remove gate), after its controls.
  actions?: ReactNode;
  framed?: boolean;
  nested?: boolean;
  disabled?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <fieldset
      className={cx(
        "ui-field-group",
        framed && "ui-field-group-framed",
        nested && "ui-field-group-nested",
        className,
      )}
      disabled={disabled}
    >
      <legend>{legend}</legend>
      {description && (
        <p className="ui-field-group-description">{description}</p>
      )}
      {children}
      {actions && <div className="ui-field-group-actions">{actions}</div>}
    </fieldset>
  );
}
