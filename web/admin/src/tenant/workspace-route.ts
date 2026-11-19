import { text, type Item } from "../api-types";
import type { TenantRoute, TenantScreen } from "./screen";
import { sectionLabel } from "./navigation";

// What the Tenant workspace shows for a route: its subject, title, heading
// and the highlighted sidebar item.

// The open Space or App (`detail`) and whether the loaded one is the
// route's.
export function routeDetail(route: TenantRoute, space: Item, app: Item) {
  const detail = route.page === "space" ? space : app,
    detailReady =
      route.page === "space" ? space.id === route.space : app.id === route.app;
  return { detail, detailReady };
}

// The People tab shown: the section's default ("general") is Members for a
// reader of the Tenant's members, otherwise Teams.
export const peopleTabOf = (
  route: TenantRoute,
  all: (action: string) => boolean,
) =>
  route.tab === "general"
    ? all("membership.read")
      ? "members"
      : "teams"
    : route.tab;

export const pageTitle = (
  route: TenantRoute,
  detail: Item,
  detailReady: boolean,
) =>
  route.page === "account"
    ? "My account"
    : ["space", "app"].includes(route.page)
      ? detailReady
        ? text(detail.display_name)
        : route.page === "space"
          ? "Space"
          : "App"
      : sectionLabel(route.page) || "Home";

// The one highlighted sidebar item: a Space's Entities and Access tabs
// belong to the Entities and Access screens, so the Space's own link is
// current only on its other tabs.
export const activeNavItem = (route: TenantRoute) =>
  route.page === "space" && route.tab === "entities"
    ? "entities"
    : (route.page === "space" && route.tab === "access") ||
        route.page === "people"
      ? "access"
      : route.page === "app"
        ? "apps"
        : route.page === "settings" && route.tab === "agents"
          ? "agents"
          : route.page;

// The shell's page heading. A Space's Access tab is headed as the Space's
// access. `entityScreen` is a Space's entity designer.
export function shellHeading(
  { route, tenant }: TenantScreen,
  title: string,
  detailReady: boolean,
  entityScreen: boolean,
) {
  const spaceAccess =
    route.page === "space" && route.tab === "access" && detailReady;
  return {
    title:
      route.page === "agents"
        ? "Agent connections"
        : spaceAccess
          ? `Access · ${title}`
          : title,
    description:
      route.page === "agents"
        ? "Agents act for a person, inside a grant. Same operations, same audit trail."
        : spaceAccess
          ? "Business roles live in the Space. Tenant administration grants no automatic record access."
          : undefined,
    // Home, Activity and an App draw their own heading (Activity's
    // filters sit beside its title); so does the entity designer.
    header:
      (!["home", "activity", "app"].includes(route.page) && !entityScreen) ||
      !tenant,
  };
}
