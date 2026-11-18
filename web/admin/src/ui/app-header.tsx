import type { ReactNode } from "react";
import { cx, initials } from "./utils";
import type { Tone } from "./status-tone";

// An App's initials on a tile whose color follows the App's status tone (not
// the App itself): "sm" in lists, "lg" in the App's own header.
export function AppTile({
  name,
  tone = "info",
  size = "sm",
}: {
  name: string;
  tone?: Tone;
  size?: "sm" | "lg";
}) {
  return (
    <span
      className={cx("ui-app-tile", "ui-tile-" + size, "tone-" + tone)}
      aria-hidden
    >
      {initials(name)}
    </span>
  );
}

// An App page's heading: its tile, the name (the page's h1) over its address,
// the status pill and right-aligned actions.
export function AppHeader({
  name,
  tone,
  address,
  status,
  actions,
}: {
  name: string;
  tone?: Tone;
  address?: ReactNode;
  status?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="ui-app-header">
      <AppTile name={name} tone={tone} size="lg" />
      <div className="ui-app-header-name">
        <h1>{name}</h1>
        {address && <span>{address}</span>}
      </div>
      {status}
      {actions && <div className="ui-app-header-actions">{actions}</div>}
    </div>
  );
}

// A titled card with its body below: the heading with a note beside it
// ("inline", e.g. the production deployment and its state) or under it
// ("stacked", e.g. Approved origins), and actions at the end of the line.
export function TitledCard({
  title,
  note,
  actions,
  layout = "inline",
  className,
  role,
  children,
}: {
  title: ReactNode;
  note?: ReactNode;
  actions?: ReactNode;
  layout?: "inline" | "stacked";
  // An e2e hook.
  className?: string;
  role?: "status";
  children?: ReactNode;
}) {
  return (
    <section
      className={cx("panel ui-titled-card", "ui-titled-" + layout, className)}
      role={role}
    >
      <header>
        <h2>{title}</h2>
        {note && <span>{note}</span>}
        {actions && <div className="ui-titled-card-actions">{actions}</div>}
      </header>
      {children}
    </section>
  );
}

// Cards side by side that share the row's height and wrap below 340 px each.
export function CardRow({ children }: { children: ReactNode }) {
  return <div className="ui-card-row">{children}</div>;
}

// A screen's blocks one above the other, 24 px apart.
export function PageStack({ children }: { children: ReactNode }) {
  return <div className="ui-page-stack">{children}</div>;
}

// A value in a bordered box (an approved origin).
export function ValueBox({ children }: { children: ReactNode }) {
  return <div className="ui-value-box">{children}</div>;
}
