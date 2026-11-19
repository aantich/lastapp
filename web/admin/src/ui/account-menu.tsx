import type { ReactNode } from "react";
import { Avatar } from "./avatar";
import { Menu, type MenuItem } from "./menu";
import { cx } from "./utils";

// The signed-in person at the bottom of the sidebar, opening the account
// menu upward: avatar, name and role (the avatar alone on the rail). The
// name stays in the accessible name on the rail.
export function AccountMenu({
  name,
  detail,
  items,
  rail,
  className,
}: {
  name: string;
  detail?: ReactNode;
  items: MenuItem[];
  rail?: boolean;
  className?: string;
}) {
  return (
    <Menu
      className={cx("ui-account", className)}
      triggerClassName="ui-account-trigger"
      placement="above"
      align="start"
      trigger={
        <>
          <Avatar name={name} size="md" className="ui-account-avatar" />
          <span className={cx("ui-account-copy", rail && "ui-sr-only")}>
            <strong>{name}</strong>
            {detail && <small>{detail}</small>}
          </span>
          <span className="ui-sr-only">, account menu</span>
        </>
      }
      items={items}
    />
  );
}
