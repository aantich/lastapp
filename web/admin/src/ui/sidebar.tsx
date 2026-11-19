import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import type { Tone } from "./status-tone";
import { Tooltip } from "./tooltip";
import { cx } from "./utils";
import { Icon } from "./icon";

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
        {items.map(({ id, label: name, icon: Glyph, badge }) => {
          const button = (
            <button
              key={id}
              type="button"
              className={cx("ui-nav-item", active === id && "selected")}
              aria-current={active === id ? "page" : undefined}
              aria-label={collapsed ? name : undefined}
              onClick={() => onSelect(id)}
            >
              <Icon icon={Glyph} size="xl" stroke="light" aria-hidden />
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
  // A short mono tag after the name (a workspace's "workspace").
  tag?: ReactNode;
};

// A titled list of places in the sidebar (the Tenant's Spaces): a status
// square, the name and a mono tag. Hidden on the collapsed rail. With
// `onTitle` the title is a button that opens the full list (`titleCurrent`
// while that list is the page shown).
export function SidebarLinks({
  label,
  title,
  onTitle,
  titleCurrent,
  items,
  active,
  onSelect,
  collapsed,
}: {
  label: string;
  title: string;
  onTitle?: () => void;
  titleCurrent?: boolean;
  items: SidebarLink[];
  active?: string;
  onSelect: (id: string) => void;
  collapsed?: boolean;
}) {
  if (collapsed) return null;
  return (
    <div className="ui-sidebar-group">
      {onTitle ? (
        <button
          type="button"
          className="ui-nav-section ui-section-trigger"
          aria-current={titleCurrent ? "page" : undefined}
          onClick={onTitle}
        >
          {title}
        </button>
      ) : (
        <span className="ui-nav-section">{title}</span>
      )}
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
