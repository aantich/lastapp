import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import type { Tone } from "./status-tone";
import { Tooltip } from "./tooltip";
import { cx } from "./utils";

export type NavItem = {
  id: string;
  label: string;
  icon: LucideIcon;
  badge?: ReactNode;
};

// The sidebar's screen list under its section label (the Tenant's name, or
// a Tenant switcher in its place). The current screen is aria-current="page".
// On the collapsed rail only the icons show; each keeps its accessible name
// and shows it as a tooltip on hover and keyboard focus.
export function SidebarNav({
  label,
  items,
  active,
  onSelect,
  collapsed,
  section,
}: {
  label: string;
  items: NavItem[];
  active: string;
  onSelect: (id: string) => void;
  collapsed?: boolean;
  section?: ReactNode;
}) {
  return (
    <div className="ui-sidebar-group">
      {section && !collapsed && section}
      <nav aria-label={label} className="nav-group">
        {items.map(({ id, label: name, icon: Icon, badge }) => {
          const button = (
            <button
              key={id}
              type="button"
              className={cx("ui-nav-item", active === id && "selected")}
              aria-current={active === id ? "page" : undefined}
              aria-label={collapsed ? name : undefined}
              onClick={() => onSelect(id)}
            >
              <Icon size={18} strokeWidth={1.8} aria-hidden />
              <span className="ui-nav-label">{name}</span>
              {badge !== undefined && !collapsed && (
                <span className="ui-nav-badge">{badge}</span>
              )}
            </button>
          );
          return collapsed ? (
            <Tooltip key={id} content={name} side="right" labelOnly>
              {button}
            </Tooltip>
          ) : (
            button
          );
        })}
      </nav>
    </div>
  );
}

export type SidebarLink = {
  id: string;
  label: string;
  // The status square before the name.
  tone: Tone;
  // A short mono tag after the name ("prod", "dev").
  tag?: ReactNode;
};

// A titled list of places in the sidebar (the Tenant's Spaces): a status
// square, the name and a mono tag. Hidden on the collapsed rail. `title` may
// be a button that opens the full list.
export function SidebarLinks({
  label,
  title,
  items,
  active,
  onSelect,
  collapsed,
}: {
  label: string;
  title: ReactNode;
  items: SidebarLink[];
  active?: string;
  onSelect: (id: string) => void;
  collapsed?: boolean;
}) {
  if (collapsed) return null;
  return (
    <div className="ui-sidebar-group">
      {title}
      <nav aria-label={label} className="nav-group">
        {items.map((item) => (
          <button
            key={item.id}
            type="button"
            className={cx("ui-nav-item", active === item.id && "selected")}
            aria-current={active === item.id ? "page" : undefined}
            onClick={() => onSelect(item.id)}
          >
            <i className={cx("ui-nav-dot", "tone-" + item.tone)} aria-hidden />
            <span className="ui-nav-label">{item.label}</span>
            {item.tag && <span className="ui-nav-tag">{item.tag}</span>}
          </button>
        ))}
      </nav>
    </div>
  );
}
