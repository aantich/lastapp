import { useEffect } from "react";
import { api } from "../api";
import { consoleURL, parseTenantURL, useConsoleURL } from "../console-url";
import type { Field, Modal } from "../ui";
import { useAction } from "../use-action";
import { useScreenError } from "../route-state";
import {
  num,
  optText,
  text,
  type Item,
  type Me,
  type TenantCapabilities,
  type TenantOverview,
} from "../api-types";
import { editFields, type TenantScreen } from "./screen";
import { accessTarget } from "./navigation";
import type { useTenantQueries } from "./use-tenant-queries";

// Builds the TenantScreen every page reads from the loaded queries: the
// resolved route, capability checks, navigation and the shared modal/action
// plumbing. It also sends a URL outside the Tenant console back to Home.
export function useTenantScreen({
  me,
  url,
  navigate,
  parsed,
  tenant,
  base,
  queries,
  setModal,
}: {
  me: Me;
  url: string;
  navigate: ReturnType<typeof useConsoleURL>[1];
  parsed: ReturnType<typeof parseTenantURL>;
  tenant: string;
  base: string;
  queries: ReturnType<typeof useTenantQueries>;
  setModal: (modal: Modal | null) => void;
}) {
  const { overviewQuery, capsQuery, spaceQuery, appQuery, spacesQuery } =
    queries;
  const overview: TenantOverview = overviewQuery.data || {},
    caps: TenantCapabilities = capsQuery.data?.permissions || {},
    space: Item = spaceQuery.data || {},
    app: Item = appQuery.data || {};
  const { error, setError, dismiss } = useScreenError(url, [
    overviewQuery.error,
    capsQuery.error,
    spaceQuery.error,
    appQuery.error,
  ]);
  const route = {
    page: parsed.page,
    // An App URL names no Space; take it from the loaded App.
    space:
      parsed.space ||
      (parsed.app && app.id === parsed.app ? text(app.space_id) || "" : ""),
    app: parsed.app,
    tab: parsed.tab,
    team: parsed.team,
  };
  if (route.page === "spaces" && route.space) route.page = "space";
  if (route.page === "apps" && route.app) route.page = "app";
  const profile = overview.tenant || {};
  const all = (action: string) => !!caps[action]?.all,
    can = (action: string) => !!caps[action]?.available;
  const scoped = (action: string, spaceID = route.space, appID = route.app) =>
    all(action) ||
    (caps[action]?.spaces || []).includes(spaceID) ||
    (caps[action]?.apps || []).includes(appID);
  const { busy, run: act } = useAction(url, {
    onSuccess: () => done(),
    onError: setError,
  });
  const sidebarSpaces = spacesQuery.data?.items || [];
  function go(page: string, v: Partial<typeof route> = {}) {
    if (page === "account") {
      navigate(consoleURL.account);
      return;
    }
    if (page === "entities") {
      const space = route.space || text(sidebarSpaces[0]?.id);
      if (space) go("space", { space, tab: "entities" });
      else go("spaces");
      return;
    }
    if (page === "access") {
      const target = accessTarget(
        { all, can, scoped },
        caps["authorization.read"],
        route,
        sidebarSpaces.map((sp) => text(sp.id)),
      );
      if (target?.page === "space")
        go("space", { space: target.space, tab: "access" });
      else if (target?.page === "app")
        go("app", { app: target.app, tab: "access" });
      else go(target?.page || "home");
      return;
    }
    const path =
      page === "space"
        ? ["spaces", v.space || route.space, v.tab || "apps"]
        : page === "app"
          ? ["apps", v.app || route.app, v.tab || "releases"]
          : page === "people"
            ? [
                "people",
                v.tab || (all("membership.read") ? "members" : "teams"),
                ...(v.team ? [v.team] : []),
              ]
            : page === "settings"
              ? ["settings", v.tab || "general"]
              : [page];
    navigate(consoleURL.tenant(tenant, ...path));
  }
  const openSpace = (r: Item) => go("space", { space: optText(r.id) }),
    openApp = (r: Item) => go("app", { app: optText(r.id) }),
    openEntity = (space: string, entity: string) =>
      navigate(
        consoleURL.tenant(tenant, "spaces", space, "entities") +
          "?entity=" +
          encodeURIComponent(entity),
      );
  useEffect(() => {
    if (parsed.area !== "tenants" && parsed.area !== "account" && tenant)
      navigate(consoleURL.tenant(tenant, "home"), true);
  }, [parsed.area, tenant, navigate]);
  function done() {
    setModal(null);
  }
  function edit(
    row: Item,
    path: string,
    title: string,
    fields: Field[] = editFields,
  ) {
    setModal({
      title,
      fields,
      initial: { ...row },
      save: (v) =>
        api(
          path,
          "PATCH",
          Object.fromEntries(
            fields.map((f) => [
              f.key,
              f.type === "number" ? Number(v[f.key]) : v[f.key],
            ]),
          ),
          num(row.revision),
        ),
    });
  }

  const t: TenantScreen = {
    me,
    tenant,
    base,
    route,
    caps,
    overview,
    profile,
    space,
    app,
    all,
    can,
    scoped,
    go,
    spaces: sidebarSpaces,
    openSpace,
    openApp,
    openEntity,
    edit,
    setModal,
    setError,
    busy,
    act,
  };
  return { t, error, dismiss, done };
}
