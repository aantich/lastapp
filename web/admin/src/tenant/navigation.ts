import {
  Activity,
  Bot,
  House,
  LayoutGrid,
  Shield,
  AlignLeft,
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

// Entities open the current (or first) Space's Entities tab, Access the
// Tenant's members and teams (page "people"), Agents the Tenant's agent
// access for a security manager and otherwise the person's own. Spaces are
// listed under the sidebar's Spaces heading; Settings is in the account menu.
const sections: (NavItem & { visible?: (c: Capabilities) => boolean })[] = [
  { id: "home", label: "Home", icon: House },
  { id: "apps", label: "Apps", icon: LayoutGrid },
  {
    id: "entities",
    label: "Entities",
    icon: AlignLeft,
    visible: (c) => c.can("space.read") && c.can("entity.manage"),
  },
  {
    id: "people",
    label: "Access",
    icon: Shield,
    visible: (c) => c.all("membership.read") || c.can("team.manage"),
  },
  { id: "agents", label: "Agents", icon: Bot },
  {
    id: "activity",
    label: "Activity",
    icon: Activity,
    visible: (c) => c.can("audit.read"),
  },
];

// Pages reached from elsewhere than the sidebar sections, by their title.
const otherPages: Record<string, string> = {
  spaces: "Spaces",
  settings: "Settings",
};

// The sections this account may open, in sidebar order; `badges` adds a
// count to a section (Agents).
export const tenantNav = (
  c: Capabilities,
  badges: Record<string, number | undefined> = {},
): NavItem[] =>
  sections
    .filter((s) => !s.visible || s.visible(c))
    .map(({ id, label, icon }) => ({ id, label, icon, badge: badges[id] }));

export const sectionLabel = (id: string) =>
  sections.find((s) => s.id === id)?.label || otherPages[id];

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
