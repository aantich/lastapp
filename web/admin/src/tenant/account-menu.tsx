import { Settings, Users } from "lucide-react";
import type { MenuItem } from "../ui";
import type { TenantScreen } from "./screen";
import { peopleVisible, type Capabilities } from "./navigation";
import { Icon } from "../ui";

// The Tenant's entries in the account menu: its members and teams (when
// this account may see them) and its settings.
export const tenantAccountItems = (
  capabilities: Capabilities,
  go: TenantScreen["go"],
): MenuItem[] => [
  ...(peopleVisible(capabilities)
    ? [
        {
          id: "people",
          label: "Members and teams",
          icon: <Icon icon={Users} size="lg" aria-hidden />,
          onSelect: () => go("people"),
        },
      ]
    : []),
  {
    id: "settings",
    label: "Tenant settings",
    icon: <Icon icon={Settings} size="lg" aria-hidden />,
    onSelect: () => go("settings"),
  },
];
