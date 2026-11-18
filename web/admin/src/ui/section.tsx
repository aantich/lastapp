import { useId, type ReactNode } from "react";
import { cx } from "./utils";

// A titled part of a page that is not itself a card ("Needs attention" above
// a grid of attention cards): a section named by its h2, with optional
// trailing actions (usually a link button to the full list).
export function Section({
  title,
  actions,
  children,
  className,
}: {
  title: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  const id = useId();
  return (
    <section className={cx("ui-section", className)} aria-labelledby={id}>
      <header className="ui-section-header">
        <h2 id={id}>{title}</h2>
        {actions && <div className="ui-section-actions">{actions}</div>}
      </header>
      {children}
    </section>
  );
}

// The muted sentence under the page header that sums up the page ("Good
// morning, Ava. 2 things need you in Northstar.").
export function Lead({ children }: { children: ReactNode }) {
  return <p className="ui-lead">{children}</p>;
}
