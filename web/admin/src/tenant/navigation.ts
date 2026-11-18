import {
  AppWindow,
  FolderOpen,
  House,
  ScrollText,
  Settings,
  Users,
} from "lucide-react";
import type { NavItem, TabItem } from "../ui";
import type { Item } from "../api-types";

// The Tenant console's navigation, the one source for the sidebar, the
// command palette and the page tabs. Visibility follows the capabilities the
// backend reports (GET …/capabilities); the backend still authorizes every
// request a screen makes.
export type Capabilities = {
  all: (action: string) => boolean;
  can: (action: string) => boolean;
  scoped: (action: string, spaceID?: string, appID?: string) => boolean;
};

const sections: (NavItem & { visible?: (c: Capabilities) => boolean })[] = [
  { id: "home", label: "Home", icon: House },
  { id: "apps", label: "Apps", icon: AppWindow },
  {
    id: "spaces",
    label: "Spaces",
    icon: FolderOpen,
    visible: (c) => c.can("space.read"),
  },
  {
    id: "people",
    label: "People",
    icon: Users,
    visible: (c) => c.all("membership.read") || c.can("team.manage"),
  },
  {
    id: "activity",
    label: "Activity",
    icon: ScrollText,
    visible: (c) => c.can("audit.read"),
  },
  { id: "settings", label: "Settings", icon: Settings },
];

// The sections this account may open, in sidebar order.
export const tenantNav = (c: Capabilities): NavItem[] =>
  sections
    .filter((s) => !s.visible || s.visible(c))
    .map(({ id, label, icon }) => ({ id, label, icon }));

export const sectionLabel = (id: string) =>
  sections.find((s) => s.id === id)?.label;

// A page's tabs: a Space or App (`space` is the open Space, for the
// development-workspace check), People or Settings.
export function tenantTabs(
  page: string,
  c: Capabilities,
  space: Item = {},
): TabItem[] {
  const tabs: [string, string, boolean?][] =
    page === "space"
      ? [
          ["apps", "Apps"],
          [
            "development",
            "Development",
            !space.workspace_id && c.scoped("workspace.manage"),
          ],
          ["entities", "Entities", c.scoped("entity.manage")],
          ["access", "Access", c.scoped("authorization.read")],
          ["settings", "Settings"],
        ]
      : page === "app"
        ? [
            ["microapps", "Micro-apps"],
            ["releases", "Releases", c.scoped("release.manage")],
            ["access", "Access", c.scoped("authorization.read")],
            ["settings", "Settings"],
          ]
        : page === "people"
          ? [
              ["members", "Members", c.all("membership.read")],
              ["teams", "Teams", c.can("team.read")],
            ]
          : page === "settings"
            ? [
                ["general", "General"],
                ["databases", "Databases", c.all("database.read")],
                ["agents", "Agent access", c.all("authorization.manage")],
              ]
            : [];
  return tabs
    .filter(([, , shown]) => shown !== false)
    .map(([id, label]) => ({ id, label }));
}
