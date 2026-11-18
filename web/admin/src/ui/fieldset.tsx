import type { ReactNode } from "react";
import { cx } from "./utils";

// A titled group of related controls inside one form ("What may this agent
// do?", "Provisioning limits"): a native fieldset, so assistive technology
// announces the legend when focus enters the group.
export function Fieldset({
  legend,
  description,
  children,
  className,
}: {
  legend: ReactNode;
  // One sentence under the legend on what the group decides.
  description?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <fieldset className={cx("ui-fieldset", className)}>
      <legend>{legend}</legend>
      {description && <p className="ui-fieldset-description">{description}</p>}
      <div className="ui-fieldset-body">{children}</div>
    </fieldset>
  );
}
