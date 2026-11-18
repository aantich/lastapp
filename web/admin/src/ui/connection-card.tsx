import { useId, type ReactNode } from "react";
import { cx } from "./utils";

// A delegated connection (an agent acting for a person): the client's
// initial on an ink tile, its name and who/where it acts for, a status at
// the end; then its chips (preset, permissions) and a muted footer line
// (expiry, last use) with actions at the end. An inactive connection
// (revoked, expired) is dimmed.
export function ConnectionCard({
  name,
  subtitle,
  status,
  chips,
  meta,
  actions,
  inactive,
  className,
}: {
  name: string;
  subtitle?: ReactNode;
  status?: ReactNode;
  chips?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
  inactive?: boolean;
  className?: string;
}) {
  return (
    <article
      className={cx(
        "panel ui-connection",
        inactive && "is-inactive",
        className,
      )}
    >
      <div className="ui-connection-head">
        <span className="ui-connection-tile" aria-hidden>
          {(name.trim()[0] || "?").toUpperCase()}
        </span>
        <span className="ui-connection-name">
          <strong>{name}</strong>
          {subtitle && <span>{subtitle}</span>}
        </span>
        {status}
      </div>
      {chips && <div className="ui-connection-chips">{chips}</div>}
      {(meta || actions) && (
        <div className="ui-connection-foot">
          {meta}
          {actions && <span className="ui-connection-actions">{actions}</span>}
        </div>
      )}
    </article>
  );
}

// Connection cards in a grid that fits as many 320 px columns as the width
// allows.
export function ConnectionGrid({ children }: { children: ReactNode }) {
  return <div className="ui-connection-grid">{children}</div>;
}

export type Choice = {
  value: string;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
};

// A few named starting points shown as cards (a label and a line under
// it): a labelled group of toggle buttons, the pressed one is current.
// Nothing may be pressed (the value matches no card).
export function ChoiceCards({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: Choice[];
  value: string;
  onChange: (value: string) => void;
}) {
  const id = useId();
  // A card is named by its label; the line under it describes it.
  return (
    <div className="ui-choice-cards" role="group" aria-label={label}>
      {options.map((o, i) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={o.value === value}
          aria-describedby={o.description ? `${id}-${i}` : undefined}
          disabled={o.disabled}
          onClick={() => onChange(o.value)}
        >
          <strong>{o.label}</strong>
          {o.description && (
            <span id={`${id}-${i}`} aria-hidden>
              {o.description}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

// A focused single-task panel (a consent request): a title, sections one
// under another and a footer of actions aligned to the end, at most
// 680 px of content wide. `level` 1 when the card is the page's heading.
export function FlowCard({
  title,
  titleId,
  level = 2,
  aside,
  children,
  footer,
  className,
}: {
  title: ReactNode;
  titleId?: string;
  level?: 1 | 2;
  // At the end of the title line (a close button, a status).
  aside?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  className?: string;
}) {
  const Heading = level === 1 ? "h1" : "h2";
  return (
    <section
      className={cx("panel ui-flow-card", className)}
      aria-labelledby={titleId}
    >
      <div className="ui-flow-card-head">
        <Heading id={titleId} className="ui-flow-title">
          {title}
        </Heading>
        {aside}
      </div>
      {children}
      {footer && <div className="ui-flow-card-foot">{footer}</div>}
    </section>
  );
}

// One part of a FlowCard: a small bold label over its content. With `id`
// the label names a group inside (aria-labelledby).
export function FlowSection({
  label,
  id,
  children,
}: {
  label: ReactNode;
  id?: string;
  children: ReactNode;
}) {
  return (
    <div className="ui-flow-section">
      <span className="ui-flow-label" id={id}>
        {label}
      </span>
      {children}
    </div>
  );
}

// FlowCard's sections without the card, where a Dialog already frames them.
export function FlowStack({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={cx("ui-flow-stack", className)}>{children}</div>;
}
