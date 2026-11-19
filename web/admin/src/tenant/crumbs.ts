import type { Crumb } from "../ui";
import { optText, text } from "../api-types";
import type { TenantScreen } from "./screen";
import { sectionLabel } from "./navigation";

// The Tenant workspace's breadcrumbs: the Tenant, then the section, Space or
// App (with its Space) being shown. `entityScreen` is a Space's entity
// designer, which sits under its Space.
export function tenantCrumbs(
  t: TenantScreen,
  {
    tenantName,
    title,
    entityScreen,
    detailReady,
  }: {
    tenantName: string;
    title: string;
    entityScreen: boolean;
    detailReady: boolean;
  },
): Crumb[] {
  const { route, app, can, scoped, go } = t;
  return [
    {
      label: tenantName || "Tenant",
      onClick: () => go("home"),
    },
    ...(entityScreen
      ? [
          {
            label: title,
            onClick: () => go("space", { space: route.space }),
          },
          { label: "Entities" },
        ]
      : route.page === "space"
        ? [
            {
              label: "Spaces",
              onClick: can("space.read") ? () => go("spaces") : undefined,
            },
            { label: title },
          ]
        : route.page === "app"
          ? [
              { label: "Apps", onClick: () => go("apps") },
              ...(detailReady
                ? [
                    {
                      label: text(app.space_name),
                      onClick: scoped("space.read", optText(app.space_id), "")
                        ? () => t.openSpace({ id: app.space_id })
                        : undefined,
                    },
                  ]
                : []),
              { label: title },
            ]
          : [{ label: sectionLabel(route.page) || title }]),
  ];
}
