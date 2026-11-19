import type { ReactNode } from "react";
import { Avatar, Inline } from "../ui";
import { text, type Item } from "../api-types";

// The Person column shared by the Tenant members and team members lists.
export const personColumn: [string, (r: Item) => ReactNode] = [
  "Person",
  (r) => (
    <Inline gap="sm">
      <Avatar name={text(r.display_name)} size={28} />
      <span>
        <strong>{text(r.display_name)}</strong> <small>{text(r.email)}</small>
      </span>
    </Inline>
  ),
];
