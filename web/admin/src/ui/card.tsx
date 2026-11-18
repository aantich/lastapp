import { useId, type HTMLAttributes, type ReactNode } from "react";
import { cx, initials } from "./utils";
import { StatusBadge } from "./status";
import type { Tone } from "./status-tone";

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

// Something that needs a person: a failed deploy, a database being
// prepared. The state drives the badge; failures get a danger border. The
// body is the title and a description (children), then actions.
export function AttentionCard({
  state,
  stateLabel,
  meta,
  title,
  children,
  extra,
  actions,
  tone,
}: {
  state: string;
  stateLabel?: ReactNode;
  meta?: ReactNode;
  title: ReactNode;
  children?: ReactNode;
  // Under the description, e.g. a Progress.
  extra?: ReactNode;
  actions?: ReactNode;
  tone?: Tone;
}) {
  const badge = <StatusBadge state={state} label={stateLabel} tone={tone} />;
  return (
    <article className="panel ui-attention">
      <div className="ui-attention-meta">
        {badge}
        {meta && <span>{meta}</span>}
      </div>
      <div className="ui-attention-body">
        <h3>{title}</h3>
        {children && <p>{children}</p>}
        {extra}
      </div>
      {actions && <div className="ui-attention-actions">{actions}</div>}
    </article>
  );
}

export function AttentionGrid({ children }: { children: ReactNode }) {
  return <div className="ui-attention-grid">{children}</div>;
}

// A one-line card for an empty attention list ("All clear").
export function AllClear({ children }: { children: ReactNode }) {
  return (
    <div className="panel ui-all-clear">
      <StatusBadge state="clear" tone="success" label="All clear" />
      <span>{children}</span>
    </div>
  );
}

const tileTones: Tone[] = ["success", "info", "warning", "neutral"];

// An App as a card that opens it: its initials tile (a tone stable for the
// App's id), name, a subtitle (Space · micro-apps), its status, a mono
// address line and a footer under a hairline (production release).
export function AppCard({
  id,
  name,
  subtitle,
  status,
  address,
  footer,
  onOpen,
}: {
  id: string;
  name: string;
  subtitle?: ReactNode;
  status?: ReactNode;
  address?: ReactNode;
  footer?: ReactNode;
  onOpen: () => void;
}) {
  const described = useId();
  let hash = 0;
  for (const c of id) hash = (hash * 31 + c.charCodeAt(0)) >>> 0;
  const tone = tileTones[hash % tileTones.length];
  // The App's name is the button (its name is exactly what it shows); the
  // button's ::after stretches over the card so the whole card opens it,
  // and the rest of the card describes it.
  return (
    <article className="panel ui-app-card">
      <span className="ui-app-card-head">
        <span className={cx("ui-app-tile", "tone-" + tone)} aria-hidden>
          {initials(name)}
        </span>
        <span className="ui-app-card-name">
          <strong>
            <button
              type="button"
              className="ui-app-card-open"
              onClick={onOpen}
              aria-describedby={described}
            >
              {name}
            </button>
          </strong>
          {subtitle && <span>{subtitle}</span>}
        </span>
        {status}
      </span>
      <span id={described} className="ui-app-card-detail">
        {address && <span className="ui-app-card-address">{address}</span>}
        {footer && <span className="ui-app-card-footer">{footer}</span>}
      </span>
    </article>
  );
}
