import type { ReactNode } from "react";

export type Crumb = {
  label: ReactNode;
  // A link target or a click handler; the last crumb is the current page.
  href?: string;
  onClick?: () => void;
};

// Where the page sits in Tenant → Space → App. The last crumb is the current
// page (aria-current) and is not a link.
export function Breadcrumbs({
  items,
  label = "Breadcrumb",
}: {
  items: Crumb[];
  label?: string;
}) {
  return (
    <nav className="ui-breadcrumbs" aria-label={label}>
      <ol>
        {items.map((item, i) => {
          const last = i === items.length - 1;
          return (
            <li key={i}>
              {last ? (
                <span aria-current="page">{item.label}</span>
              ) : item.href ? (
                <a href={item.href}>{item.label}</a>
              ) : item.onClick ? (
                <button type="button" onClick={item.onClick}>
                  {item.label}
                </button>
              ) : (
                <span>{item.label}</span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

// The page's own heading block: one h1 per page, an optional description, a
// mono meta line (IDs, hosts) and right-aligned actions.
export function PageHeader({
  title,
  description,
  meta,
  actions,
  breadcrumbs,
}: {
  title: ReactNode;
  description?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
  breadcrumbs?: ReactNode;
}) {
  return (
    <div className="ui-page-header">
      <div className="ui-page-header-copy">
        {breadcrumbs}
        <h1>{title}</h1>
        {meta && <div className="ui-page-header-meta">{meta}</div>}
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="ui-page-header-actions">{actions}</div>}
    </div>
  );
}

// A dashboard's opening: a muted eyebrow (the date), a large greeting h1 and
// a summary sentence, with compact stats beside it.
export function PageIntro({
  eyebrow,
  title,
  summary,
  aside,
}: {
  eyebrow?: ReactNode;
  title: ReactNode;
  summary?: ReactNode;
  aside?: ReactNode;
}) {
  return (
    <section className="ui-page-intro">
      <div className="ui-page-intro-copy">
        {eyebrow && <span>{eyebrow}</span>}
        <h1>{title}</h1>
        {summary && <p>{summary}</p>}
      </div>
      {aside}
    </section>
  );
}
