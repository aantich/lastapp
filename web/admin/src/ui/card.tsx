import type { HTMLAttributes, ReactNode } from "react";
import { cx } from "./utils";
import { StatusBadge } from "./status";
import { statusTone, type Tone } from "./status-tone";

// The console card surface (.panel). Use the parts for a titled card; a bare
// Card is fine for custom layouts.
export function Card({
  as: As = "section",
  className,
  ...rest
}: HTMLAttributes<HTMLElement> & { as?: "section" | "article" | "div" }) {
  return <As className={cx("panel", className)} {...rest} />;
}

export function CardHeader({
  title,
  description,
  actions,
  level = 2,
  id,
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  level?: 2 | 3;
  id?: string;
}) {
  const Heading = level === 2 ? "h2" : "h3";
  return (
    <header className="ui-card-header">
      <div>
        <Heading id={id}>{title}</Heading>
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="row-actions">{actions}</div>}
    </header>
  );
}

export function CardBody({
  className,
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return <div className={cx("ui-card-body", className)} {...rest} />;
}

export function CardFooter({
  className,
  ...rest
}: HTMLAttributes<HTMLElement>) {
  return <footer className={cx("ui-card-footer", className)} {...rest} />;
}

// A KPI tile. With onClick it is a button (e.g. opens the filtered list).
export function StatCard({
  label,
  value,
  detail,
  onClick,
}: {
  label: ReactNode;
  value: ReactNode;
  detail?: ReactNode;
  onClick?: () => void;
}) {
  const content = (
    <>
      <span>{label}</span>
      <strong>{value}</strong>
      {detail && <small>{detail}</small>}
    </>
  );
  return onClick ? (
    <button type="button" className="panel ui-stat" onClick={onClick}>
      {content}
    </button>
  ) : (
    <div className="panel ui-stat">{content}</div>
  );
}

export function StatGrid({ children }: { children: ReactNode }) {
  return <div className="ui-stat-grid">{children}</div>;
}

// Something that needs a person: a failed deploy, a pending review. The state
// drives the badge; failures get a danger border.
export function AttentionCard({
  state,
  stateLabel,
  meta,
  title,
  children,
  actions,
  tone,
}: {
  state: string;
  stateLabel?: ReactNode;
  meta?: ReactNode;
  title: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  tone?: Tone;
}) {
  const badge = <StatusBadge state={state} label={stateLabel} tone={tone} />;
  const danger = (tone || statusTone(state)) === "danger";
  return (
    <article className={cx("panel ui-attention", danger && "tone-danger")}>
      <div className="ui-attention-meta">
        {badge}
        {meta && <span>{meta}</span>}
      </div>
      <h3>{title}</h3>
      {children && <p>{children}</p>}
      {actions && <div className="ui-attention-actions">{actions}</div>}
    </article>
  );
}

export function AttentionGrid({ children }: { children: ReactNode }) {
  return <div className="ui-attention-grid">{children}</div>;
}
