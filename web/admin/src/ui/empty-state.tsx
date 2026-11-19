import type { ReactNode } from "react";
import { FolderOpen } from "lucide-react";
import { cx } from "./utils";
import { Icon } from "./icon";

// What a list or page shows when it has nothing yet: an icon, a short title,
// one sentence on what to do and, optionally, the action that does it.
export function EmptyState({
  icon = <Icon icon={FolderOpen} size="6xl" aria-hidden />,
  title,
  children,
  actions,
  compact,
}: {
  icon?: ReactNode;
  title?: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  compact?: boolean;
}) {
  return (
    <div className={cx("empty", compact && "compact")}>
      {icon}
      {title && <h3>{title}</h3>}
      {children && <p>{children}</p>}
      {actions && <div className="ui-empty-actions">{actions}</div>}
    </div>
  );
}
