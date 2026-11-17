import { useEffect } from "react";
import {
  AppWindow,
  FolderOpen,
  Users,
  ScrollText,
  Settings,
  Plus,
  RefreshCw,
} from "lucide-react";
import { api } from "./api";
import { DevelopmentBanner } from "./development-workspaces";
import { ConsoleShell } from "./console-shell";
import { consoleURL, parseTenantURL, useConsoleURL } from "./console-url";
import { FormModal, type Field, type Modal } from "./form-modal";
import { useAppearance } from "./appearance";
import { useAction } from "./use-action";
import { RemoteSelect } from "./remote-select";
import { TenantPeople } from "./tenant-people";
import { readPreference, savePreference } from "./preferences";
import { LiveRefresh, refreshQueries, useApiQuery } from "./query";
import { useRouteState, useScreenError } from "./route-state";
import { AccountPage } from "./account-page";
import { ActivityLog } from "./activity-log";
import { ErrorBoundary } from "./error-boundary";
import {
  num,
  optText,
  text,
  type Item,
  type Me,
  type TenantCapabilities,
  type TenantOverview,
} from "./api-types";
import { editFields, options, type TenantScreen } from "./tenant/screen";
import {
  addDatabase,
  createApp,
  createMicro,
  createSpace,
} from "./tenant/forms";
import { AppsPage, SpacesPage } from "./tenant/inventory";
import { SpacePage } from "./tenant/space-page";
import { AppPage } from "./tenant/app-page";
import { TenantSettings } from "./tenant/settings-page";

// The Tenant console: resolves the Tenant and route, loads what every page
// shares (overview, capabilities and the open Space or App) and composes the
// pages under tenant/.
const nav = [
  { id: "apps", label: "Apps", icon: AppWindow },
  { id: "spaces", label: "Spaces", icon: FolderOpen },
  { id: "people", label: "People", icon: Users },
  { id: "activity", label: "Activity", icon: ScrollText },
  { id: "settings", label: "Settings", icon: Settings },
];
export function TenantWorkspace({
  me,
  appearanceControls,
  signOut,
  refreshMe,
}: {
  me: Me;
  appearanceControls: ReturnType<typeof useAppearance>;
  signOut: () => void;
  refreshMe: () => Promise<void>;
}) {
  const memberships = me.tenants || [],
    [url, navigate] = useConsoleURL(),
    parsed = parseTenantURL(url);
  const saved = readPreference("lastapp-tenant"),
    fallback = memberships.some((t) => t.id === saved)
      ? saved
      : memberships[0]?.id || "";
  const tenant = parsed.area === "tenants" ? parsed.tenant : fallback,
    base = "/v1/tenants/" + tenant;
  const [modal, setModal] = useRouteState<Modal | null>(url, null);
  // Everything this workspace shows refreshes every 5 s while no form is open.
  // refetchInterval never overlaps requests and pauses in a hidden tab.
  const live = tenant && !modal ? 5000 : false;
  const overviewQuery = useApiQuery<TenantOverview>(
      tenant ? base + "/overview" : null,
      { refetchInterval: live },
    ),
    capsQuery = useApiQuery<{ permissions: TenantCapabilities }>(
      tenant ? base + "/capabilities" : null,
      { refetchInterval: live },
    ),
    // A Space or App screen also reads its subject.
    spaceQuery = useApiQuery<Item>(
      tenant && parsed.page === "spaces" && parsed.space
        ? base + "/spaces/" + parsed.space
        : null,
      { refetchInterval: live },
    ),
    appQuery = useApiQuery<Item>(
      tenant && parsed.page === "apps" && parsed.app
        ? base + "/apps/" + parsed.app
        : null,
      { refetchInterval: live },
    );
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
  const refresh = () => void refreshQueries();
  const { busy, run: act } = useAction(url, {
    onSuccess: () => done(),
    onError: setError,
  });
  function go(page: string, v: Partial<typeof route> = {}) {
    if (page === "account") {
      navigate(consoleURL.account);
      return;
    }
    const path =
      page === "space"
        ? ["spaces", v.space || route.space, v.tab || "apps"]
        : page === "app"
          ? ["apps", v.app || route.app, v.tab || "microapps"]
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
  function selectTenant(id: string) {
    if (!id) return;
    savePreference("lastapp-tenant", id);
    navigate(consoleURL.tenant(id, "apps"));
  }
  const openSpace = (r: Item) => go("space", { space: optText(r.id) }),
    openApp = (r: Item) => go("app", { app: optText(r.id) });
  useEffect(() => {
    if (parsed.area !== "tenants" && parsed.area !== "account" && tenant)
      navigate(consoleURL.tenant(tenant, "apps"), true);
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
    openSpace,
    openApp,
    edit,
    setModal,
    setError,
    busy,
    act,
  };
  const detail = route.page === "space" ? space : app,
    detailReady =
      route.page === "space" ? space.id === route.space : app.id === route.app;
  const peopleTab =
    route.tab === "general"
      ? all("membership.read")
        ? "members"
        : "teams"
      : route.tab;
  const tabs =
    route.page === "space"
      ? [
          ["apps", "Apps"],
          ...(!space.workspace_id && scoped("workspace.manage")
            ? [["development", "Development"]]
            : []),
          ...(scoped("entity.manage") ? [["entities", "Entities"]] : []),
          ...(scoped("authorization.read") ? [["access", "Access"]] : []),
          ["settings", "Settings"],
        ]
      : route.page === "app"
        ? [
            ["microapps", "Micro-apps"],
            ...(scoped("release.manage") ? [["releases", "Releases"]] : []),
            ...(scoped("authorization.read") ? [["access", "Access"]] : []),
            ["settings", "Settings"],
          ]
        : route.page === "people"
          ? [
              ...(all("membership.read") ? [["members", "Members"]] : []),
              ...(can("team.read") ? [["teams", "Teams"]] : []),
            ]
          : route.page === "settings"
            ? [
                ["general", "General"],
                ...(all("database.read") ? [["databases", "Databases"]] : []),
                ...(all("authorization.manage")
                  ? [["agents", "Agent access"]]
                  : []),
              ]
            : [];
  const title =
    route.page === "account"
      ? "My account"
      : ["space", "app"].includes(route.page)
        ? detailReady
          ? text(detail.display_name)
          : route.page === "space"
            ? "Space"
            : "App"
        : nav.find((p) => p.id === route.page)?.label || "Apps";
  const menu = nav
    .filter((n) => n.id !== "spaces" || can("space.read"))
    .filter(
      (n) => n.id !== "people" || all("membership.read") || can("team.manage"),
    )
    .filter((n) => n.id !== "activity" || can("audit.read"));
  const canCreateApp =
    all("app.manage") || (caps["app.manage"]?.spaces || []).length > 0;
  const actions = (
    <>
      <button className="icon" aria-label="Refresh" onClick={refresh}>
        <RefreshCw size={16} />
      </button>
      {route.page === "spaces" && all("space.manage") && (
        <button className="primary" onClick={() => void createSpace(t)}>
          <Plus size={16} />
          Create Space
        </button>
      )}
      {((route.page === "apps" && canCreateApp && (overview.spaces ?? 0) > 0) ||
        (route.page === "space" &&
          route.tab === "apps" &&
          scoped("app.manage") &&
          detailReady)) && (
        <button className="primary" onClick={() => createApp(t)}>
          <Plus size={16} />
          Create App
        </button>
      )}
      {route.page === "app" &&
        route.tab === "microapps" &&
        scoped("microapp.manage") &&
        detailReady && (
          <button className="primary" onClick={() => createMicro(t)}>
            <Plus size={16} />
            Add micro-app
          </button>
        )}
      {route.page === "settings" &&
        route.tab === "databases" &&
        all("database.manage") && (
          <button className="primary" onClick={() => void addDatabase(t)}>
            Add database
          </button>
        )}
    </>
  );
  return (
    <LiveRefresh value={live}>
      <ConsoleShell
        me={me}
        kind="Tenant"
        title={title}
        nav={menu}
        active={
          route.page === "space"
            ? "spaces"
            : route.page === "app"
              ? "apps"
              : route.page
        }
        navigate={go}
        context={
          memberships.length > 1 || me.pagination?.has_more ? (
            <RemoteSelect
              path="/v1/my-tenants"
              label="Tenant"
              value={tenant}
              onChange={selectTenant}
              initial={options(memberships)}
            />
          ) : (
            <strong className="company-name">
              {text(profile.display_name) ||
                memberships[0]?.display_name ||
                "No Tenant access"}
            </strong>
          )
        }
        breadcrumbs={
          <>
            <button className="text-button" onClick={() => go("apps")}>
              {text(profile.display_name) || "Tenant"}
            </button>
            {route.page === "app" && detailReady && (
              <>
                <span> / </span>
                <button
                  className="text-button"
                  disabled={!scoped("space.read", optText(app.space_id), "")}
                  onClick={() => openSpace({ id: app.space_id })}
                >
                  {text(app.space_name)}
                </button>
              </>
            )}
          </>
        }
        actions={actions}
        appearanceControls={appearanceControls}
        account={() => go("account")}
        signOut={signOut}
      >
        {error && (
          <div className="error" role="alert">
            {error}
            <button
              className="tiny"
              aria-label="Dismiss error"
              onClick={dismiss}
            >
              ×
            </button>
          </div>
        )}
        {detailReady &&
          (route.page === "space" || route.page === "app") &&
          text(detail.workspace_id) && (
            <DevelopmentBanner
              workspace={text(detail.workspace_id)}
              openSpace={(id) => go("space", { space: id })}
            />
          )}
        {!!tabs.length && (
          <div className="tabs workspace-tabs">
            {tabs.map(([id, label]) => (
              <button
                key={id}
                className={
                  (route.page === "people" ? peopleTab : route.tab) === id
                    ? "active"
                    : ""
                }
                onClick={() =>
                  go(route.page, {
                    space: route.space,
                    app: route.app,
                    tab: id,
                  })
                }
              >
                {label}
              </button>
            ))}
          </div>
        )}
        <ErrorBoundary key={url}>
          {route.page === "account" ? (
            <AccountPage me={me} refreshMe={refreshMe} />
          ) : !tenant ? (
            <section className="panel empty">
              <p>Your administrator can add this account to a Tenant.</p>
            </section>
          ) : (
            <>
              {route.page === "apps" && <AppsPage t={t} />}
              {route.page === "spaces" && <SpacesPage t={t} />}
              {route.page === "space" && detailReady && <SpacePage t={t} />}
              {route.page === "app" && detailReady && <AppPage t={t} />}
              {route.page === "people" && (
                <TenantPeople
                  key={tenant + "/" + peopleTab + "/" + route.team}
                  tenant={tenant}
                  caps={caps}
                  tab={peopleTab}
                  team={route.team}
                  selectTeam={(id) => go("people", { tab: "teams", team: id })}
                />
              )}
              {route.page === "activity" && (
                <ActivityLog path={base + "/audit-events"} />
              )}
              {route.page === "settings" && <TenantSettings t={t} />}
            </>
          )}
        </ErrorBoundary>
        {modal && (
          <FormModal
            key={url + modal.title}
            modal={modal}
            lookups={{}}
            close={() => setModal(null)}
            done={done}
          />
        )}
      </ConsoleShell>
    </LiveRefresh>
  );
}
