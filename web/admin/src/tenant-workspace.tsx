import { useEffect, type ReactNode } from "react";
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
import {
  DevelopmentWorkspaces,
  DevelopmentBanner,
} from "./development-workspaces";
import { AppReleases } from "./app-releases";
import { EntityDesigner } from "./entity-designer/designer";
import { AppDataAccess } from "./entity-designer/app-access";
import { Collection } from "./collection";
import { ConsoleShell } from "./console-shell";
import { consoleURL, parseTenantURL, useConsoleURL } from "./console-url";
import { FormModal, type Field, type Modal } from "./form-modal";
import { PasswordScreen } from "./login";
import { useAppearance } from "./appearance";
import { errorMessage, useAction } from "./use-action";
import { confirmAction } from "./confirm";
import { RemoteSelect } from "./remote-select";
import { AccessControl } from "./access-control";
import { TenantPeople } from "./tenant-people";
import { readPreference, savePreference } from "./preferences";
import { LiveRefresh, refreshQueries, useApiQuery } from "./query";
import { useRouteState, useScreenError } from "./route-state";
import { nameFields, stateOptions, status as badge } from "./console-forms";
import type {
  Item,
  Me,
  Page,
  TenantCapabilities,
  TenantOverview,
} from "./api-types";
const states = stateOptions;
const editFields: Field[] = [
  { key: "display_name", label: "Name", required: true },
  {
    key: "state",
    label: "Status",
    type: "select",
    options: states,
    required: true,
  },
];
function computeField(dedicated: boolean): Field {
  return {
    key: "compute_mode",
    label: "Compute preference",
    type: "select",
    required: true,
    advanced: true,
    options: [
      { value: "shared", label: "Shared" },
      ...(dedicated ? [{ value: "dedicated", label: "Dedicated" }] : []),
    ],
  };
}
const options = (items: Item[], key = "id", label = "display_name") =>
  items.map((x) => ({ value: x[key], label: x[label] }));
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
      (parsed.app && app.id === parsed.app ? app.space_id || "" : ""),
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
  const openSpace = (r: Item) => go("space", { space: r.id }),
    openApp = (r: Item) => go("app", { app: r.id });
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
          row.revision,
        ),
    });
  }
  // setModal is bound to the route it was read on, so a lookup that finishes
  // after the user navigates away opens nothing on the new screen.
  async function createSpace() {
    try {
      const [targets, members] = await Promise.all([
        api<Page>(base + "/database-targets"),
        api<Page>(base + "/memberships"),
      ]);
      setModal({
        title: "Create Space",
        submit: "Create Space",
        description:
          "A Space owns shared business objects, rules and a database.",
        initial: {
          id: crypto.randomUUID(),
          mode: profile.allow_managed_database ? "managed" : "existing",
          layout:
            profile.default_layout ||
            (profile.allow_shared_database ? "shared" : "dedicated"),
          target_id: "",
          initial_manager_user_id: me.id,
        },
        fields: [
          ...nameFields,
          {
            key: "initial_manager_user_id",
            label: "Initial Space manager",
            type: "select",
            advanced: true,
            lookup: base + "/memberships",
            options: options(members.items, "user_id"),
            required: true,
          },
          {
            key: "mode",
            label: "Database",
            advanced: !!profile.allow_managed_database,
            type: "select",
            required: true,
            options: [
              ...(profile.allow_managed_database
                ? [{ value: "managed", label: "Create managed database" }]
                : []),
              { value: "existing", label: "Use an existing database resource" },
            ],
          },
          {
            key: "resource_id",
            label: "Existing database",
            type: "select",
            lookup: base + "/database-resources",
            required: true,
            when: (v) => v.mode === "existing",
          },
          {
            key: "layout",
            label: "New database layout",
            advanced: true,
            type: "select",
            required: true,
            options: [
              ...(profile.allow_shared_database
                ? [{ value: "shared", label: "Shared within this Tenant" }]
                : []),
              ...(profile.allow_dedicated_database
                ? [{ value: "dedicated", label: "Dedicated to this Space" }]
                : []),
            ],
            when: (v) => v.mode === "managed",
          },
          {
            key: "target_id",
            label: "Managed database target",
            advanced: true,
            type: "select",
            lookup: base + "/database-targets?managed=true",
            options: options(
              targets.items.filter((t: Item) => t.managed_enabled),
            ),
            when: (v) => v.mode === "managed",
          },
        ],
        save: (v) =>
          api(base + "/spaces", "POST", {
            id: v.id,
            slug: v.slug,
            display_name: v.display_name,
            initial_manager_user_id: v.initial_manager_user_id,
            database:
              v.mode === "existing"
                ? { resource_id: v.resource_id }
                : {
                    mode: "managed",
                    layout: v.layout,
                    target_id: v.target_id || "",
                  },
          }),
      });
    } catch (e) {
      setError(errorMessage(e));
    }
  }
  function createApp() {
    setModal({
      title: "Create App",
      submit: "Create App",
      description:
        route.space === space.id && space.workspace_id
          ? "Develop this App in your isolated workspace. Production capacity is checked when you deploy."
          : "The App reuses its Space’s database and definitions.",
      initial: {
        id: crypto.randomUUID(),
        initial_manager_user_id: me.id,
        compute_mode: "shared",
        space_id: route.space,
      },
      fields: [
        ...nameFields,
        ...(!route.space
          ? [
              {
                key: "space_id",
                label: "Space",
                type: "select",
                lookup: base + "/spaces",
                required: true,
              },
            ]
          : []),
        {
          key: "initial_manager_user_id",
          label: "Initial App manager",
          advanced: true,
          when: () => !!route.space,
          type: "select",
          options: [{ value: me.id, label: me.display_name }],
          lookup: route.space
            ? base + "/spaces/" + route.space + "/recipients"
            : base + "/recipients",
          required: true,
          hint: "An active Tenant member.",
        },
        computeField(!!profile.allow_dedicated_compute),
      ],
      save: (v) =>
        api(base + `/spaces/${v.space_id || route.space}/apps`, "POST", {
          id: v.id,
          slug: v.slug,
          display_name: v.display_name,
          initial_manager_user_id: v.initial_manager_user_id,
          compute_mode: v.compute_mode,
        }),
    });
  }
  function createMicro() {
    setModal({
      title: "Add micro-app",
      submit: "Add micro-app",
      description: "Record a client experience within this App.",
      initial: { id: crypto.randomUUID(), navigation_order: 0 },
      fields: [
        ...nameFields,
        { key: "artifact_ref", label: "Artifact reference", advanced: true },
        {
          key: "navigation_order",
          label: "Navigation order",
          type: "number",
          advanced: true,
        },
      ],
      save: (v) =>
        api(base + `/apps/${route.app}/microapps`, "POST", {
          ...v,
          navigation_order: Number(v.navigation_order),
        }),
    });
  }
  async function addDatabase() {
    try {
      const targets = await api<Page>(base + "/database-targets");
      setModal({
        title: "Add database",
        submit: "Add database",
        initial: {
          id: crypto.randomUUID(),
          management_mode: profile.allow_managed_database
            ? "managed"
            : "registered",
          layout:
            profile.default_layout ||
            (profile.allow_shared_database ? "shared" : "dedicated"),
          target_id: targets.items[0]?.id,
        },
        fields: [
          {
            key: "management_mode",
            label: "Method",
            type: "select",
            required: true,
            options: [
              ...(profile.allow_managed_database
                ? [{ value: "managed", label: "Create managed database" }]
                : []),
              ...(profile.allow_registered_database
                ? [{ value: "registered", label: "Register existing database" }]
                : []),
            ],
          },
          {
            key: "target_id",
            label: "Database target",
            type: "select",
            lookup: base + "/database-targets",
            options: options(targets.items),
            required: true,
          },
          {
            key: "layout",
            label: "Layout",
            type: "select",
            required: true,
            options: [
              ...(profile.allow_shared_database
                ? [{ value: "shared", label: "Shared within Tenant" }]
                : []),
              ...(profile.allow_dedicated_database
                ? [{ value: "dedicated", label: "Dedicated to one Space" }]
                : []),
            ],
          },
          {
            key: "database_name",
            label: "Existing database name",
            required: true,
            when: (v) => v.management_mode === "registered",
          },
          {
            key: "username",
            label: "Existing database username",
            required: true,
            when: (v) => v.management_mode === "registered",
          },
          {
            key: "password",
            label: "Existing database password",
            type: "password",
            required: true,
            when: (v) => v.management_mode === "registered",
          },
        ],
        save: (v) =>
          api(base + "/database-resources", "POST", {
            id: v.id,
            management_mode: v.management_mode,
            layout: v.layout,
            target_id: v.target_id,
            ...(v.management_mode === "registered"
              ? {
                  database_name: v.database_name,
                  credentials: { username: v.username, password: v.password },
                }
              : {}),
          }),
      });
    } catch (e) {
      setError(errorMessage(e));
    }
  }

  const nameColumn: [string, (r: Item) => ReactNode] = [
      "Name",
      (r) => <strong>{r.display_name}</strong>,
    ],
    statusColumn: [string, (r: Item) => ReactNode] = [
      "Status",
      (r) => badge(r.state),
    ];
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
          ? detail.display_name
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
        <button className="primary" onClick={createSpace}>
          <Plus size={16} />
          Create Space
        </button>
      )}
      {((route.page === "apps" && canCreateApp && (overview.spaces ?? 0) > 0) ||
        (route.page === "space" &&
          route.tab === "apps" &&
          scoped("app.manage") &&
          detailReady)) && (
        <button className="primary" onClick={createApp}>
          <Plus size={16} />
          Create App
        </button>
      )}
      {route.page === "app" &&
        route.tab === "microapps" &&
        scoped("microapp.manage") &&
        detailReady && (
          <button className="primary" onClick={createMicro}>
            <Plus size={16} />
            Add micro-app
          </button>
        )}
      {route.page === "settings" &&
        route.tab === "databases" &&
        all("database.manage") && (
          <button className="primary" onClick={addDatabase}>
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
              {profile.display_name ||
                memberships[0]?.display_name ||
                "No Tenant access"}
            </strong>
          )
        }
        breadcrumbs={
          <>
            <button className="text-button" onClick={() => go("apps")}>
              {profile.display_name || "Tenant"}
            </button>
            {route.page === "app" && detailReady && (
              <>
                <span> / </span>
                <button
                  className="text-button"
                  disabled={!scoped("space.read", app.space_id, "")}
                  onClick={() => openSpace({ id: app.space_id })}
                >
                  {app.space_name}
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
          detail.workspace_id && (
            <DevelopmentBanner
              workspace={detail.workspace_id}
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
        {route.page === "account" ? (
          <>
            <AgentAccess me={me} />
            <section className="panel workspace-section">
              <p>
                {me.display_name} · {me.email}
              </p>
            </section>
            {me.auth_method === "password" ? (
              <PasswordScreen done={refreshMe} />
            ) : (
              <p className="panel workspace-section">
                Manage your password with your single sign-on provider.
              </p>
            )}
          </>
        ) : !tenant ? (
          <section className="panel empty">
            <p>Your administrator can add this account to a Tenant.</p>
          </section>
        ) : (
          <>
            {route.page === "apps" && (
              <>
                {overview.spaces === 0 && all("space.manage") && (
                  <section className="panel first-space">
                    <div>
                      <h2>Create your first Space</h2>
                      <p>A Space gives your Apps a shared database.</p>
                    </div>
                    <button className="primary" onClick={createSpace}>
                      Create Space
                    </button>
                  </section>
                )}
                <Collection
                  key={base + "apps"}
                  path={base + "/apps"}
                  columns={[
                    nameColumn,
                    ["Space", (r) => r.space_name],
                    statusColumn,
                  ]}
                  empty="No Apps are available yet."
                  actions={(r) => (
                    <button className="text-button" onClick={() => openApp(r)}>
                      Open
                    </button>
                  )}
                />
              </>
            )}
            {route.page === "spaces" && (
              <Collection
                key={base + "spaces"}
                path={base + "/spaces"}
                columns={[
                  nameColumn,
                  ["Apps", (r) => r.app_count],
                  ["Database", (r) => badge(r.database_state)],
                  statusColumn,
                ]}
                empty="Create a Space to organize your Apps."
                actions={(r) => (
                  <button className="text-button" onClick={() => openSpace(r)}>
                    Open
                  </button>
                )}
              />
            )}
            {route.page === "space" && detailReady && (
              <>
                {route.tab === "development" && !space.workspace_id && (
                  <DevelopmentWorkspaces
                    space={space.id}
                    openSpace={(id) => go("space", { space: id })}
                  />
                )}
                {route.tab === "apps" && (
                  <Collection
                    key={space.id + "apps"}
                    path={base + "/apps?space_id=" + space.id}
                    columns={[
                      nameColumn,
                      ["Micro-apps", (r) => r.microapp_count],
                      statusColumn,
                    ]}
                    actions={(r) => (
                      <button
                        className="text-button"
                        onClick={() => openApp(r)}
                      >
                        Open
                      </button>
                    )}
                  />
                )}
                {route.tab === "entities" && scoped("entity.manage") && (
                  <EntityDesigner
                    key={space.id}
                    space={space.id}
                    tenant={tenant}
                    me={me}
                  />
                )}
                {route.tab === "access" && (
                  <AccessControl
                    key={space.id}
                    tenant={tenant}
                    caps={caps}
                    scopeKind="space"
                    scopeId={space.id}
                  />
                )}
                {route.tab === "settings" && (
                  <>
                    <section className="panel workspace-section">
                      <div className="settings-heading">
                        <div className="settings-summary">
                          <h2>General</h2>
                          {badge(space.state)}
                        </div>
                        {scoped("space.manage") && (
                          <button
                            className="secondary"
                            onClick={() =>
                              edit(
                                space,
                                base + "/spaces/" + space.id,
                                "Edit Space",
                              )
                            }
                          >
                            Edit Space
                          </button>
                        )}
                      </div>
                    </section>
                    <section className="panel workspace-section">
                      <h2 className="section-title">Database</h2>
                      <dl className="workspace-facts">
                        <dt>Connection</dt>
                        <dd>{badge(space.database_state)}</dd>
                        <dt>Layout</dt>
                        <dd>{space.database_layout}</dd>
                      </dl>
                      {all("database.read") && (
                        <button
                          className="secondary"
                          onClick={() => go("settings", { tab: "databases" })}
                        >
                          Manage databases
                        </button>
                      )}
                      <details>
                        <summary>Advanced</summary>
                        <p>Database: {space.database_name}</p>
                        <p>Slug: {space.slug}</p>
                      </details>
                    </section>
                  </>
                )}
              </>
            )}
            {route.page === "app" && detailReady && (
              <>
                {route.tab === "releases" && (
                  <AppReleases
                    key={app.id}
                    app={app}
                    tenant={tenant}
                    refresh={refresh}
                    openWorkspaces={() =>
                      go("space", { space: app.space_id, tab: "development" })
                    }
                  />
                )}
                {route.tab === "microapps" && (
                  <Collection
                    key={app.id + "microapps"}
                    path={base + "/microapps?app_id=" + app.id}
                    columns={[nameColumn, statusColumn]}
                    empty="Add your first micro-app."
                    actions={
                      scoped("microapp.manage")
                        ? (r) => (
                            <button
                              className="text-button"
                              onClick={() =>
                                edit(
                                  r,
                                  base + "/microapps/" + r.id,
                                  "Edit micro-app",
                                  [
                                    {
                                      key: "display_name",
                                      label: "Name",
                                      required: true,
                                    },
                                    {
                                      key: "state",
                                      label: "Status",
                                      type: "select",
                                      options: states,
                                      required: true,
                                    },
                                    {
                                      key: "artifact_ref",
                                      label: "Artifact reference",
                                      advanced: true,
                                    },
                                    {
                                      key: "navigation_order",
                                      label: "Navigation order",
                                      type: "number",
                                      advanced: true,
                                    },
                                  ],
                                )
                              }
                            >
                              Edit micro-app
                            </button>
                          )
                        : undefined
                    }
                  />
                )}
                {route.tab === "access" && (
                  <AccessControl
                    key={app.id}
                    tenant={tenant}
                    caps={caps}
                    scopeKind="app"
                    scopeId={app.id}
                  />
                )}
                {route.tab === "settings" && (
                  <>
                    <section className="panel workspace-section">
                      <div className="settings-heading">
                        <div className="settings-summary">
                          <h2>General</h2>
                          {badge(app.state)}
                        </div>
                        {scoped("app.manage") && (
                          <button
                            className="secondary"
                            onClick={() =>
                              edit(app, base + "/apps/" + app.id, "Edit App", [
                                ...editFields,
                                computeField(
                                  !!profile.allow_dedicated_compute ||
                                    app.compute_mode === "dedicated",
                                ),
                              ])
                            }
                          >
                            Edit App
                          </button>
                        )}
                      </div>
                      <details>
                        <summary>Advanced</summary>
                        <p>Slug: {app.slug}</p>
                        <p>Compute preference: {app.compute_mode}</p>
                      </details>
                    </section>
                    {scoped("app.manage") && (
                      <AppDataAccess
                        key={app.id}
                        app={app.id}
                        spaceManager={scoped("entity.manage", app.space_id, "")}
                      />
                    )}
                  </>
                )}
              </>
            )}
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
              <Collection
                path={base + "/audit-events"}
                columns={[
                  [
                    "Event",
                    (r) => (
                      <>
                        <strong>{r.action}</strong>
                        <small>{r.resource_type}</small>
                      </>
                    ),
                  ],
                  ["Actor", (r) => r.actor_name],
                  ["Time", (r) => new Date(r.created_at).toLocaleString()],
                ]}
              />
            )}
            {route.page === "settings" && route.tab === "agents" && (
              <AgentAccess key={tenant} me={me} tenant={tenant} />
            )}
            {route.page === "settings" && route.tab === "general" && (
              <section className="panel workspace-section">
                <dl className="workspace-facts">
                  <dt>Name</dt>
                  <dd>{profile.display_name}</dd>
                  <dt>Plan</dt>
                  <dd>{profile.plan_name}</dd>
                  <dt>Status</dt>
                  <dd>{profile.state}</dd>
                  <dt>Spaces / Apps / Databases</dt>
                  <dd>
                    {profile.max_spaces ?? "Unlimited"} /{" "}
                    {profile.max_apps ?? "Unlimited"} /{" "}
                    {profile.max_databases ?? "Unlimited"}
                  </dd>
                </dl>
                {all("tenant.manage") && (
                  <button
                    className="secondary"
                    onClick={() =>
                      edit(profile, base, "Edit Tenant", [
                        { key: "display_name", label: "Name", required: true },
                      ])
                    }
                  >
                    Edit Tenant
                  </button>
                )}
              </section>
            )}
            {route.page === "settings" && route.tab === "databases" && (
              <Collection
                path={base + "/database-resources"}
                columns={[
                  [
                    "Database",
                    (r) => (
                      <>
                        <strong>{r.database_name}</strong>
                        <small>
                          {r.management_mode} · {r.layout}
                        </small>
                      </>
                    ),
                  ],
                  statusColumn,
                  ["Last result", (r) => r.last_error_message || "—"],
                ]}
                actions={
                  all("database.manage")
                    ? (r) => (
                        <>
                          {["ready", "failed"].includes(r.state) && (
                            <button
                              className="text-button"
                              disabled={busy}
                              onClick={() =>
                                act(() =>
                                  api(
                                    base +
                                      "/database-resources/" +
                                      r.id +
                                      "/retry",
                                    "POST",
                                    {},
                                    r.revision,
                                  ),
                                )
                              }
                            >
                              {r.state === "failed"
                                ? "Retry"
                                : "Recheck connection"}
                            </button>
                          )}
                          {r.management_mode === "registered" &&
                            !["provisioning", "retired"].includes(r.state) && (
                              <button
                                className="text-button"
                                onClick={() =>
                                  setModal({
                                    title: "Update database credentials",
                                    submit: "Save",
                                    initial: {},
                                    fields: [
                                      {
                                        key: "username",
                                        label: "Username",
                                        required: true,
                                      },
                                      {
                                        key: "password",
                                        label: "Password",
                                        type: "password",
                                        required: true,
                                      },
                                    ],
                                    save: (v) =>
                                      api(
                                        base + "/database-resources/" + r.id,
                                        "PATCH",
                                        {
                                          credentials: {
                                            username: v.username,
                                            password: v.password,
                                          },
                                        },
                                        r.revision,
                                      ),
                                  })
                                }
                              >
                                Update credentials
                              </button>
                            )}
                          {r.management_mode === "registered" &&
                            r.state === "ready" && (
                              <button
                                className="text-button"
                                onClick={() =>
                                  setModal({
                                    title: "Schema credentials",
                                    submit: "Save",
                                    initial: {},
                                    fields: [
                                      {
                                        key: "username",
                                        label: "Schema username",
                                        required: true,
                                      },
                                      {
                                        key: "password",
                                        label: "Schema password",
                                        type: "password",
                                        required: true,
                                      },
                                    ],
                                    save: (v) =>
                                      api(
                                        base +
                                          "/database-resources/" +
                                          r.id +
                                          "/schema-credentials",
                                        "POST",
                                        v,
                                        r.revision,
                                      ),
                                  })
                                }
                              >
                                Set schema credentials
                              </button>
                            )}
                          {!["provisioning", "retired"].includes(r.state) && (
                            <button
                              className="text-button danger"
                              disabled={busy}
                              onClick={async () => {
                                if (
                                  await confirmAction({
                                    title: "Retire database resource?",
                                    message:
                                      "Retire this database resource? Physical data is preserved.",
                                    confirmLabel: "Retire",
                                    danger: true,
                                  })
                                )
                                  act(() =>
                                    api(
                                      base + "/database-resources/" + r.id,
                                      "PATCH",
                                      { state: "retired" },
                                      r.revision,
                                    ),
                                  );
                              }}
                            >
                              Retire
                            </button>
                          )}
                        </>
                      )
                    : undefined
                }
              />
            )}
          </>
        )}
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
import { AgentAccess } from "./agent-access";
