import type { Key, ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { cx } from "./utils";
import { Icon } from "./icon";

export type Fact = { label: ReactNode; value: ReactNode; mono?: boolean };

// Labelled values of one record (database, build, release): a description
// list whose terms and values stay paired as rows.
export function FactList({
  facts,
  className,
}: {
  facts: Fact[];
  className?: string;
}) {
  return (
    <dl className={cx("ui-facts", className)}>
      {facts.map((f, i) => (
        <div key={i}>
          <dt>{f.label}</dt>
          <dd className={f.mono ? "ui-mono" : undefined}>{f.value}</dd>
        </div>
      ))}
    </dl>
  );
}

// Secondary detail a person opens on demand ("Advanced", an entity's
// fields): a native details/summary, so it works by keyboard and is
// announced as expandable.
export function Disclosure({
  summary,
  meta,
  children,
  defaultOpen,
  className,
}: {
  summary: ReactNode;
  // Muted text after the summary ("12 fields · 3 permissions").
  meta?: ReactNode;
  children: ReactNode;
  defaultOpen?: boolean;
  className?: string;
}) {
  return (
    <details className={cx("ui-disclosure", className)} open={defaultOpen}>
      <summary>
        <Icon icon={ChevronRight} size="sm" aria-hidden />
        <span>{summary}</span>
        {meta && <small>{meta}</small>}
      </summary>
      <div className="ui-disclosure-body">{children}</div>
    </details>
  );
}

// Preformatted machine text (a manifest, a configuration snapshot): a
// scrollable, focusable region named by its label.
export function CodeBlock({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <pre className="ui-code" role="region" aria-label={label} tabIndex={0}>
      {children}
    </pre>
  );
}

export type PickItem = {
  id: Key;
  title: ReactNode;
  meta?: ReactNode;
  // Trailing state, usually a StatusBadge.
  badge?: ReactNode;
};

// A list of records where one is open beside it (release history): each
// row is a toggle button, the open one pressed. `className`/`itemClassName`
// carry screen hooks; `footer` holds "Load more".
export function PickList({
  label,
  items,
  value,
  onPick,
  empty,
  footer,
  className,
  itemClassName,
  disabled,
}: {
  label: string;
  items: PickItem[];
  value?: Key;
  onPick: (id: Key) => void;
  empty?: ReactNode;
  footer?: ReactNode;
  className?: string;
  itemClassName?: string;
  disabled?: boolean;
}) {
  return (
    <div className={cx("ui-pick-list", className)}>
      {items.length ? (
        <ul aria-label={label}>
          {items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className={cx("ui-pick-item", itemClassName)}
                aria-pressed={item.id === value}
                disabled={disabled}
                onClick={() => onPick(item.id)}
              >
                <span>
                  <strong>{item.title}</strong>
                  {item.meta && <small>{item.meta}</small>}
                </span>
                {item.badge}
              </button>
            </li>
          ))}
        </ul>
      ) : (
        empty && <p className="ui-pick-empty">{empty}</p>
      )}
      {footer}
    </div>
  );
}
