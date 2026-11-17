import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Avatar, type AvatarKind } from "./avatar";
import { Tooltip } from "./tooltip";

export type NavItem = {
  id: string;
  label: string;
  icon: LucideIcon;
  badge?: ReactNode;
};

// The sidebar's screen list. The current screen is aria-current="page". On
// the collapsed rail each item keeps its accessible name and shows it as a
// tooltip on hover and keyboard focus.
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
    <nav aria-label={label}>
      {section && <div className="ui-nav-section">{section}</div>}
      <div className="nav-group">
        {items.map(({ id, label: name, icon: Icon, badge }) => {
          const button = (
            <button
              key={id}
              type="button"
              className={active === id ? "selected" : ""}
              aria-current={active === id ? "page" : undefined}
              aria-label={collapsed ? name : undefined}
              onClick={() => onSelect(id)}
            >
              <Icon size={19} aria-hidden />
              <span>{name}</span>
              {badge !== undefined && (
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
      </div>
    </nav>
  );
}

// The signed-in identity at the bottom of the sidebar (avatar only on the
// rail).
export function SidebarFooter({
  name,
  detail,
  kind = "person",
}: {
  name: string;
  detail?: ReactNode;
  kind?: AvatarKind;
}) {
  return (
    <div className="ui-sidebar-footer">
      <Avatar name={name} kind={kind} />
      <span>
        <strong>{name}</strong>
        {detail && <small>{detail}</small>}
      </span>
    </div>
  );
}
