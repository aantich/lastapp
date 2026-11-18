import { useEffect, useState } from "react";
import {
  Activity as ActivityIcon,
  AppWindow,
  Bot,
  Database,
  FolderOpen,
  GitBranch,
  Plus,
  RefreshCw,
  RotateCcw,
  Settings,
} from "lucide-react";
import { api } from "./api";
import { DevelopmentBanner } from "./development-workspaces";
import { workspacesPath } from "./workspaces";
import { ConsoleShell } from "./console-shell";
import { consoleURL, parseTenantURL, useConsoleURL } from "./console-url";
import {
  Alert,
  Button,
  Card,
  ContextLabel,
  ContextSwitcher,
  EmptyState,
  SidebarLinks,
  statusTone,
  ErrorBoundary,
  FormModal,
  IconButton,
  Tabs,
  type Command,
  type Crumb,
  type Field,
  type Modal,
} from "./ui";
import { useAppearance } from "./appearance";
import { useAction } from "./use-action";
import { TenantPeople } from "./tenant-people";
import { readPreference, savePreference } from "./preferences";
import {
  cachedData,
  cachedRows,
  LiveRefresh,
  refreshQueries,
  useApiQueries,
  useApiQuery,
} from "./query";
import { useRouteState, useScreenError } from "./route-state";
import { AccountPage } from "./account-page";
import { ActivityLog } from "./activity-log";
import { activityQuery, agentGrantsPath, auditActors } from "./actors";
import { retryDeployment, useDeploymentStates } from "./deployment";
import { ConnectAgentDialog } from "./connect-agent";
import { AgentAccess } from "./agent-access";
import {
  num,
  optText,
  text,
  type AgentGrant,
  type Deployment,
  type Item,
  type Me,
  type Page,
  type TenantCapabilities,
  type TenantOverview,
} from "./api-types";
import { editFields, options, type TenantScreen } from "./tenant/screen";
import { addDatabase, createApp, createSpace } from "./tenant/forms";
import { AppsPage, SpacesPage } from "./tenant/inventory";
import { HomePage } from "./tenant/home-page";
import { sectionLabel, tenantNav, tenantTabs } from "./tenant/navigation";
import { SpacePage } from "./tenant/space-page";
import { AppPage } from "./tenant/app-page";
import { TenantSettings } from "./tenant/settings-page";

// The Tenant console: resolves the Tenant and route, loads what every page
// shares (overview, capabilities and the open Space or App) and composes the
// pages under tenant/. Navigation comes from tenant/navigation.ts.
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
  const [connecting, setConnecting] = useState(false);
  const deploymentStates = useDeploymentStates();
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
  // The sidebar's Spaces (the first page) and its Agents count (active,
  // unexpired grants: every grant in the Tenant for a security manager,
  // otherwise the person's own, as Home counts them).
  const canSpaces = !!capsQuery.data?.permissions["space.read"]?.available,
    manager = !!capsQuery.data?.permissions["authorization.manage"]?.all;
  const spacesQuery = useApiQuery<Page>(
      tenant && canSpaces ? base + "/spaces?limit=8" : null,
      { refetchInterval: live },
    ),
    grantsQuery = useApiQuery<Page<AgentGrant>>(
      tenant && capsQuery.data ? agentGrantsPath(tenant, manager) : null,
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
  function selectTenant(id: string) {
    if (!id) return;
    savePreference("lastapp-tenant", id);
    navigate(consoleURL.tenant(id, "home"));
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
  const detail = route.page === "space" ? space : app,
    detailReady =
      route.page === "space" ? space.id === route.space : app.id === route.app;
  const peopleTab =
    route.tab === "general"
      ? all("membership.read")
        ? "members"
        : "teams"
      : route.tab;
  const capabilities = { all, can, scoped };
  const tabs = tenantTabs(route.page, capabilities, space);
  const activeTab = route.page === "people" ? peopleTab : route.tab;
  const title =
    route.page === "account"
      ? "My account"
      : ["space", "app"].includes(route.page)
        ? detailReady
          ? text(detail.display_name)
          : route.page === "space"
            ? "Space"
            : "App"
        : sectionLabel(route.page) || "Home";
  // Active, unexpired grants, measured when the grants were read.
  const activeAgents = grantsQuery.data
    ? grantsQuery.data.items.filter(
        (g) =>
          g.tenant_id === tenant &&
          g.state === "active" &&
          new Date(g.expires_at).getTime() > grantsQuery.dataUpdatedAt,
      ).length
    : undefined;
  const menu = tenantNav(capabilities, { agents: activeAgents });
  const tenantName =
    text(profile.display_name) ||
    memberships.find((m) => m.id === tenant)?.display_name ||
    "";
  // The Space's entity designer is its own screen (the Entities section):
  // it draws its own header and list, so the Space header and tabs give way.
  const entityScreen = route.page === "space" && route.tab === "entities";
  const crumbs: Crumb[] = [
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
                        ? () => openSpace({ id: app.space_id })
                        : undefined,
                    },
                  ]
                : []),
              { label: title },
            ]
          : [{ label: sectionLabel(route.page) || title }]),
  ];
  // The palette's commands beyond the sidebar sections: entities, Apps and
  // Spaces this session has already loaded, actions with a real operation
  // (retrying a failed deployment Home has shown, connecting an agent), the
  // Activity filters, this page's tabs and the other sections' tabs. Nothing
  // is fetched for the palette.
  const commands = (): Command[] => {
    const subject = route.page === "space" || route.page === "app" ? title : "";
    const tabCommands = (page: string, label: string, group: string) =>
      tenantTabs(page, capabilities, space).map(({ id, label: tab }) => ({
        id: `tab:${page}:${id}`,
        label: `${label} › ${String(tab)}`,
        group,
        run: () => go(page, { space: route.space, app: route.app, tab: id }),
      }));
    const apps = new Map<string, Item>();
    for (const a of [
      ...(overview.recent_apps || []),
      ...cachedRows((p) => p === base + "/apps"),
    ])
      apps.set(text(a.id), a);
    const entities = cachedRows((p) =>
      /^\/v1\/spaces\/[^/]+\/entities$/.test(p),
    );
    const failed = cachedData<{ deployment: Deployment | null }>((p) =>
      /^\/v1\/apps\/[^/]+\/deployment$/.test(p),
    ).flatMap(([path, data]) => {
      const a = apps.get(path.split("/")[3]),
        d = data?.deployment;
      return a &&
        d?.state === deploymentStates.failed &&
        scoped("release.manage", text(a.space_id), text(a.id))
        ? [{ a, d }]
        : [];
    });
    return [
      ...entities.map((e) => ({
        id: "entity:" + text(e.id),
        label: `Open ${text(e.label)} entity`,
        group: "Entities",
        keywords: text(e.entity_key),
        hint: text(e.entity_key),
        icon: <Database size={16} />,
        run: () => openEntity(text(e.space_id), text(e.id)),
      })),
      ...[...apps.values()].map((a) => ({
        id: "app:" + text(a.id),
        label: `Open ${text(a.display_name)}`,
        group: "Apps",
        keywords: text(a.space_name),
        hint: text(a.space_name),
        icon: <AppWindow size={16} />,
        run: () => openApp(a),
      })),
      ...cachedRows((p) => p === base + "/spaces").map((sp) => ({
        id: "space:" + text(sp.id),
        label: `Open ${text(sp.display_name)}`,
        group: "Spaces",
        icon: <FolderOpen size={16} />,
        run: () => openSpace(sp),
      })),
      ...failed.map(({ a, d }) => ({
        id: "retry:" + d.id,
        label: `Retry ${text(a.display_name)} deploy`,
        group: "Action",
        keywords: d.release_name,
        icon: <RotateCcw size={16} />,
        run: () => retryDeployment(d, setModal),
      })),
      {
        id: "action:connect-agent",
        label: "Connect a new agent",
        group: "Action",
        icon: <Bot size={16} />,
        run: () => setConnecting(true),
      },
      ...(menu.some((m) => m.id === "activity")
        ? auditActors
            .filter((a) => a.value)
            .map((a) => ({
              id: "activity:" + a.value,
              label: `Activity · ${a.label.toLowerCase()} only`,
              group: "Activity",
              icon: <ActivityIcon size={16} />,
              run: () =>
                navigate(
                  consoleURL.tenant(tenant, "activity") +
                    activityQuery(a.value),
                ),
            }))
        : []),
      ...(subject && detailReady
        ? tabCommands(route.page, subject, "This page")
        : []),
      ...["people", "settings"]
        .filter((page) => menu.some((m) => m.id === page))
        .flatMap((page) =>
          tabCommands(page, sectionLabel(page) || page, "Sections"),
        ),
    ];
  };
  const paletteCommands = commands();
  // Words that find real commands: an entity's key, "deploy" when a retry is
  // offered, "agent".
  const paletteExamples = [
    paletteCommands.find((c) => c.group === "Entities")?.keywords,
    paletteCommands.some((c) => c.id.startsWith("retry:")) && "deploy",
    "agent",
  ].filter((w): w is string => !!w);
  const canCreateApp =
    all("app.manage") || (caps["app.manage"]?.spaces || []).length > 0;
  const actions = (
    <>
      {route.page !== "apps" && (
        <IconButton
          label="Refresh"
          icon={<RefreshCw size={16} aria-hidden />}
          tooltip="bottom"
          onClick={refresh}
        />
      )}
      {route.page === "apps" && canCreateApp && (overview.spaces ?? 0) > 0 && (
        <Button onClick={() => createApp(t)}>New app</Button>
      )}
      {route.page === "spaces" && all("space.manage") && (
        <Button
          variant="primary"
          icon={<Plus size={16} aria-hidden />}
          onClick={() => void createSpace(t)}
        >
          Create Space
        </Button>
      )}
      {route.page === "space" &&
        route.tab === "apps" &&
        scoped("app.manage") &&
        detailReady && (
          <Button
            variant="primary"
            icon={<Plus size={16} aria-hidden />}
            onClick={() => createApp(t)}
          >
            Create App
          </Button>
        )}
      {route.page === "agents" && (
        <Button variant="primary" onClick={() => setConnecting(true)}>
          New connection
        </Button>
      )}
      {route.page === "settings" &&
        route.tab === "databases" &&
        all("database.manage") && (
          <Button variant="primary" onClick={() => void addDatabase(t)}>
            Add database
          </Button>
        )}
    </>
  );
  // The Tenant switcher is always shown when the account has a Tenant, even
  // just one, so switching is discoverable: a control under the brand row
  // that pages /v1/my-tenants. Without one, the section label over the
  // screens says so.
  const switchable = !!memberships.length || !!me.pagination?.has_more;
  const tenantSwitcher = switchable
    ? (rail: boolean) => (
        <ContextSwitcher
          kind="Tenant"
          variant="sidebar"
          rail={rail}
          className="company-name"
          path="/v1/my-tenants"
          value={tenant}
          current={tenantName}
          options={options(memberships)}
          onChange={selectTenant}
        />
      )
    : undefined;
  const tenantSection = switchable ? undefined : (
    <ContextLabel
      className="company-name"
      value={tenantName || "No Tenant access"}
    />
  );
  // Each listed Space's development workspaces this account may manage,
  // tagged "workspace" under their Space; Spaces themselves carry no tag.
  // Every row keeps its status dot.
  const workspaceParents = sidebarSpaces
    .map((sp) => text(sp.id))
    .filter((id) => scoped("workspace.manage", id, ""));
  const workspaceQueries = useApiQueries<Page>(
    workspaceParents.map(workspacesPath),
  );
  const places = sidebarSpaces.flatMap((sp) => [
    {
      id: text(sp.id),
      label: text(sp.display_name),
      tone: statusTone(text(sp.database_state || sp.state)),
    },
    ...(
      workspaceQueries[workspaceParents.indexOf(text(sp.id))]?.data?.items || []
    )
      .filter(
        (w) =>
          !!w.backing_space_id &&
          !["archived", "deleted"].includes(text(w.state)),
      )
      .map((w) => ({
        id: text(w.backing_space_id),
        label: text(w.display_name),
        tone: statusTone(text(w.state)),
        tag: (
          <>
            <GitBranch size={12} aria-hidden />
            workspace
          </>
        ),
      })),
  ]);
  // A Space's Access tab is headed as the Space's access.
  const spaceAccess =
    route.page === "space" && route.tab === "access" && detailReady;
  return (
    <LiveRefresh value={live}>
      <ConsoleShell
        me={me}
        role={me.email}
        title={
          route.page === "agents"
            ? "Agent connections"
            : spaceAccess
              ? `Access · ${title}`
              : title
        }
        description={
          route.page === "agents"
            ? "Agents act for a person, inside a grant. Same operations, same audit trail."
            : spaceAccess
              ? "Business roles live in the Space. Tenant administration grants no automatic record access."
              : undefined
        }
        // Home, Activity and an App draw their own heading (Activity's
        // filters sit beside its title); so does the entity designer.
        header={
          (!["home", "activity", "app"].includes(route.page) &&
            !entityScreen) ||
          !tenant
        }
        nav={menu}
        active={
          route.page === "space" && route.tab === "entities"
            ? "entities"
            : route.page === "app"
              ? "apps"
              : route.page === "settings" && route.tab === "agents"
                ? "agents"
                : route.page
        }
        navigate={go}
        section={tenantSection}
        context={tenantSwitcher}
        places={
          tenant && canSpaces
            ? (rail, close) => (
                <SidebarLinks
                  label="Spaces"
                  collapsed={rail}
                  title={
                    <button
                      type="button"
                      className="ui-nav-section ui-section-trigger"
                      aria-current={
                        route.page === "spaces" ? "page" : undefined
                      }
                      onClick={() => {
                        go("spaces");
                        close();
                      }}
                    >
                      Spaces
                    </button>
                  }
                  items={places}
                  active={route.page === "space" ? route.space : undefined}
                  onSelect={(id) => {
                    go("space", { space: id });
                    close();
                  }}
                />
              )
            : undefined
        }
        breadcrumbs={crumbs}
        commands={paletteCommands}
        paletteExamples={paletteExamples}
        actions={actions}
        topActions={
          <Button variant="primary" onClick={() => setConnecting(true)}>
            Connect agent
          </Button>
        }
        accountItems={[
          {
            id: "settings",
            label: "Tenant settings",
            icon: <Settings size={16} aria-hidden />,
            onSelect: () => go("settings"),
          },
        ]}
        appearanceControls={appearanceControls}
        account={() => go("account")}
        signOut={signOut}
      >
        {connecting && (
          <ConnectAgentDialog
            tenantName={tenantName}
            agentAccess={() => {
              setConnecting(false);
              go("agents");
            }}
            close={() => setConnecting(false)}
          />
        )}
        {error && (
          <Alert onDismiss={dismiss} dismissLabel="Dismiss error">
            {error}
          </Alert>
        )}
        {detailReady &&
          (route.page === "space" || route.page === "app") &&
          text(detail.workspace_id) && (
            <DevelopmentBanner
              workspace={text(detail.workspace_id)}
              openSpace={(id) => go("space", { space: id })}
            />
          )}
        {!!tabs.length && !entityScreen && route.page !== "app" && (
          <Tabs
            label={`${title} sections`}
            className="workspace-tabs"
            items={tabs}
            value={activeTab}
            onChange={(id) =>
              go(route.page, { space: route.space, app: route.app, tab: id })
            }
          />
        )}
        <ErrorBoundary key={url}>
          {route.page === "account" ? (
            <AccountPage me={me} refreshMe={refreshMe} />
          ) : !tenant ? (
            <Card>
              <EmptyState title="No Tenant yet">
                Your administrator can add this account to a Tenant.
              </EmptyState>
            </Card>
          ) : (
            <>
              {route.page === "home" && <HomePage t={t} />}
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
                <ActivityLog
                  path={base + "/audit-events"}
                  grants={agentGrantsPath(tenant, all("authorization.manage"))}
                  title={title}
                  actions={actions}
                />
              )}
              {route.page === "agents" && (
                // A Tenant security manager reviews every connection in
                // the Tenant; anyone else their own.
                <AgentAccess
                  key={tenant}
                  me={me}
                  tenant={manager ? tenant : undefined}
                  heading={false}
                />
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
