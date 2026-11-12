import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  AppWindow,
  Building2,
  Check,
  ChevronRight,
  Database,
  FolderOpen,
  Home,
  Layers3,
  LoaderCircle,
  LogOut,
  Menu,
  Moon,
  Palette,
  Plus,
  RefreshCw,
  ScrollText,
  Search,
  Settings,
  Sun,
  Users,
  X,
} from "lucide-react";
import { api } from "./api";
import { AccessControl } from "./access-control";
import { AppearancePanel, useAppearance, useDialogFocus } from "./appearance";
import { FormModal, type Field, type Modal } from "./form-modal";
import { PasswordScreen } from "./login";
import { RemoteSelect } from "./remote-select";
import { readPreference, savePreference } from "./preferences";

type Item = Record<string, any>;
type Route = { page: string; space: string; app: string; tab: string };
const homeRoute: Route = { page: "home", space: "", app: "", tab: "overview" };
const nameFields: Field[] = [
  { key: "display_name", label: "Name", required: true },
  {
    key: "slug",
    label: "Slug",
    required: true,
    hint: "Lowercase letters, numbers and hyphens.",
  },
];
const states = ["active", "suspended", "archived"].map((value) => ({
  value,
  label: value,
}));
const badge = (state: string) => (
  <span className={"badge " + state}>
    <i />
    {state}
  </span>
);
const options = (items: Item[], key = "id", label = "display_name") =>
  items.map((x) => ({ value: x[key], label: x[label] }));
const pages = [
  { id: "home", label: "Home", icon: Home },
  { id: "spaces", label: "Spaces", icon: FolderOpen },
  { id: "people", label: "People", icon: Users },
  { id: "activity", label: "Activity", icon: ScrollText },
  { id: "settings", label: "Settings", icon: Settings },
];

// All Tenant collection screens share server search/paging and scope-filtered APIs.
function Collection({
  path,
  reload,
  columns,
  actions,
  empty = "No records yet.",
}: {
  path: string;
  reload: number;
  columns: [string, (row: Item) => ReactNode][];
  actions?: (row: Item) => ReactNode;
  empty?: string;
}) {
  const [items, setItems] = useState<Item[]>([]),
    [query, setQuery] = useState(""),
    [page, setPage] = useState(1),
    [more, setMore] = useState(false),
    [loading, setLoading] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    setPage(1);
    setQuery("");
    setItems([]);
  }, [path]);
  useEffect(() => {
    let live = true;
    setLoading(true);
    setError("");
    const timer = setTimeout(() => {
      api(
        path +
          (path.includes("?") ? "&" : "?") +
          `page=${page}&q=${encodeURIComponent(query)}`,
      )
        .then((v) => {
          if (live) {
            setItems(v.items);
            setMore(!!v.pagination?.has_more);
          }
        })
        .catch((e) => {
          if (live) setError(e.message);
        })
        .finally(() => {
          if (live) setLoading(false);
        });
    }, 150);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [path, page, query, reload]);
  return (
    <section className="panel list-panel">
      <div className="toolbar">
        <label className="search">
          <Search size={16} />
          <input
            aria-label="Search records"
            placeholder="Search records…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setPage(1);
            }}
          />
        </label>
        <span>{loading ? "Loading…" : `${items.length} records`}</span>
      </div>
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {columns.map(([label]) => (
                <th key={label}>{label}</th>
              ))}
              {actions && <th>Actions</th>}
            </tr>
          </thead>
          <tbody>
            {items.map((row) => (
              <tr key={row.id || row.user_id}>
                {columns.map(([label, render]) => (
                  <td key={label}>{render(row)}</td>
                ))}
                {actions && (
                  <td>
                    <div className="row-actions">{actions(row)}</div>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {!loading && !items.length && (
        <div className="empty">
          <FolderOpen size={28} />
          <p>{empty}</p>
        </div>
      )}
      <div className="pagination">
        <button
          className="secondary"
          aria-label="Previous records"
          disabled={loading || page === 1}
          onClick={() => setPage((p) => p - 1)}
        >
          Previous
        </button>
        <span>Page {page}</span>
        <button
          className="secondary"
          aria-label="Next records"
          disabled={loading || !more}
          onClick={() => setPage((p) => p + 1)}
        >
          Next
        </button>
      </div>
    </section>
  );
}

export function TenantWorkspace({
  me,
  appearanceControls,
  signOut,
  refreshMe,
}: {
  me: Item;
  appearanceControls: ReturnType<typeof useAppearance>;
  signOut: () => void;
  refreshMe: () => Promise<void>;
}) {
  const memberships: Item[] = me.tenants || [];
  const [tenant, setTenant] = useState(() => {
    const saved = readPreference("lastapp-tenant");
    return memberships.some((t) => t.id === saved)
      ? saved
      : memberships[0]?.id || "";
  });
  const [route, setRoute] = useState<Route>(homeRoute);
  const [overview, setOverview] = useState<Item>({}),
    [caps, setCaps] = useState<Item>({}),
    [space, setSpace] = useState<Item>({}),
    [app, setApp] = useState<Item>({});
  const [reload, setReload] = useState(0),
    [error, setError] = useState(""),
    [toast, setToast] = useState("");
  const [modal, setModal] = useState<Modal | null>(null);
  const [appearanceOpen, setAppearanceOpen] = useState(false),
    [navOpen, setNavOpen] = useState(false),
    [collapsed, setCollapsed] = useState(false),
    [mobile, setMobile] = useState(
      () => matchMedia("(max-width: 800px)").matches,
    );
  const { appearance, setAppearance, isDark, reset } = appearanceControls;
  const navRef = useDialogFocus(() => setNavOpen(false), mobile && navOpen);
  const base = `/v1/tenants/${tenant}`,
    profile = overview.tenant || {};
  const contextKey = [
    tenant,
    route.page,
    route.space,
    route.app,
    route.tab,
  ].join(":");
  const currentContext = useRef(contextKey);
  currentContext.current = contextKey;
  const all = (action: string) => !!caps[action]?.all;
  const can = (action: string) => !!caps[action]?.available;
  const scoped = (action: string, spaceID = route.space, appID = route.app) =>
    all(action) ||
    (caps[action]?.spaces || []).includes(spaceID) ||
    (caps[action]?.apps || []).includes(appID);
  const refresh = () => setReload((r) => r + 1);
  function go(page: string, values: Partial<Route> = {}) {
    setRoute({ ...homeRoute, page, ...values });
    setModal(null);
    setError("");
    setNavOpen(false);
  }
  const openSpace = (row: Item) => go("space", { space: row.id });
  const openApp = (row: Item) =>
    go("app", { space: row.space_id, app: row.id });
  function selectTenant(id: string) {
    currentContext.current = "";
    setTenant(id);
    savePreference("lastapp-tenant", id);
    setOverview({});
    setCaps({});
    setSpace({});
    setApp({});
    go("home");
  }
  useEffect(() => {
    const media = matchMedia("(max-width: 800px)");
    const change = () => {
      setMobile(media.matches);
      setNavOpen(false);
    };
    media.addEventListener("change", change);
    return () => media.removeEventListener("change", change);
  }, []);
  useEffect(() => {
    if (!tenant) return;
    let live = true;
    Promise.all([api(base + "/overview"), api(base + "/capabilities")])
      .then(([o, c]) => {
        if (live) {
          setOverview(o);
          setCaps(c.permissions);
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [tenant, reload]);
  useEffect(() => {
    // Keep mounted editors and their revision snapshots during refreshes.
    setSpace((old) => (old.id === route.space ? old : {}));
    setApp((old) => (old.id === route.app ? old : {}));
    if (!tenant || !["space", "app"].includes(route.page)) return;
    let live = true;
    api(
      base +
        (route.page === "space"
          ? "/spaces/" + route.space
          : "/apps/" + route.app),
    )
      .then((v) => {
        if (live) {
          if (route.page === "space") setSpace(v);
          else setApp(v);
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [tenant, route.page, route.space, route.app, reload]);
  useEffect(() => {
    if (!tenant || modal) return;
    const timer = setInterval(refresh, 5000);
    return () => clearInterval(timer);
  }, [tenant, modal]);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 4000);
    return () => clearTimeout(timer);
  }, [toast]);
  function done() {
    setModal(null);
    refresh();
    setToast("Changes saved successfully");
  }
  async function act(fn: () => Promise<any>) {
    const started = contextKey;
    try {
      await fn();
      if (currentContext.current === started) done();
    } catch (e) {
      if (currentContext.current === started) setError((e as Error).message);
    }
  }
  function edit(
    row: Item,
    path: string,
    title: string,
    fields: Field[] = [
      { key: "display_name", label: "Name", required: true },
      {
        key: "state",
        label: "State",
        type: "select",
        options: states,
        required: true,
      },
    ],
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
  async function createSpace() {
    const started = contextKey;
    try {
      const [targets, members] = await Promise.all([
        api(base + "/database-targets"),
        api(base + "/memberships"),
      ]);
      if (currentContext.current !== started) return;
      setModal({
        title: "Create a Space",
        description:
          "A Space owns shared business objects, rules and a database.",
        initial: {
          id: crypto.randomUUID(),
          mode: profile.allow_managed_database ? "managed" : "existing",
          layout: profile.allow_shared_database ? "shared" : "dedicated",
          target_id: targets.items.find((t: Item) => t.managed_enabled)?.id,
          initial_manager_user_id: me.id,
        },
        fields: [
          ...nameFields,
          {
            key: "initial_manager_user_id",
            label: "Initial Space manager",
            type: "select",
            lookup: base + "/memberships",
            options: options(members.items, "user_id"),
            required: true,
          },
          {
            key: "mode",
            label: "Database",
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
            type: "select",
            lookup: base + "/database-targets",
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
      setError((e as Error).message);
    }
  }
  function createApp() {
    setModal({
      title: "Create an App",
      description: `An App in ${space.display_name} reuses this Space's database and definitions.`,
      initial: {
        id: crypto.randomUUID(),
        initial_manager_user_id: me.id,
        compute_mode: "shared",
      },
      fields: [
        ...nameFields,
        {
          key: "initial_manager_user_id",
          label: "Initial App manager",
          type: all("membership.read") ? "select" : "text",
          lookup: all("membership.read") ? base + "/memberships" : undefined,
          required: true,
          hint: "An active Tenant member.",
        },
        {
          key: "compute_mode",
          label: "Compute preference",
          type: "select",
          required: true,
          options: [
            { value: "shared", label: "Shared" },
            ...(profile.allow_dedicated_compute
              ? [{ value: "dedicated", label: "Dedicated" }]
              : []),
          ],
        },
      ],
      save: (v) =>
        api(base + `/spaces/${route.space}/apps`, "POST", {
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
      title: "Register a micro-app",
      description: "Record a client experience within this App.",
      initial: { id: crypto.randomUUID(), navigation_order: 0 },
      fields: [
        ...nameFields,
        { key: "artifact_ref", label: "Artifact reference" },
        { key: "navigation_order", label: "Navigation order", type: "number" },
      ],
      save: (v) =>
        api(base + `/apps/${route.app}/microapps`, "POST", {
          ...v,
          navigation_order: Number(v.navigation_order),
        }),
    });
  }
  function addMember() {
    setModal({
      title: "Add member",
      description:
        "Add an existing account by email or user ID. A platform administrator creates new global accounts and supplies their initial passwords.",
      initial: { account_mode: "existing" },
      fields: [
        {
          key: "account_mode",
          label: "Account",
          type: "select",
          required: true,
          options: [
            { value: "existing", label: "Existing account by email" },
            { value: "id", label: "Existing user by ID (including SSO)" },
          ],
        },
        {
          key: "user_id",
          label: "User ID",
          required: true,
          when: (v) => v.account_mode === "id",
        },
        {
          key: "email",
          label: "Email",
          type: "email",
          required: true,
          when: (v) => v.account_mode !== "id",
        },
      ],
      save: (v) =>
        api(
          base + "/memberships",
          "POST",
          v.account_mode === "id"
            ? { user_id: v.user_id }
            : {
                account: {
                  email: v.email,
                },
              },
        ),
    });
  }
  async function addDatabase() {
    const started = contextKey;
    try {
      const targets = await api(base + "/database-targets");
      if (currentContext.current !== started) return;
      setModal({
        title: "Add database",
        initial: {
          id: crypto.randomUUID(),
          management_mode: profile.allow_managed_database
            ? "managed"
            : "registered",
          layout: profile.allow_shared_database ? "shared" : "dedicated",
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
      setError((e as Error).message);
    }
  }

  const nameColumn: [string, (r: Item) => ReactNode] = [
    "Name",
    (r) => (
      <div className="entity">
        <span className="avatar">
          <FolderOpen size={17} />
        </span>
        <span>
          <strong>{r.display_name}</strong>
          <small>{r.slug || r.email}</small>
        </span>
      </div>
    ),
  ];
  const statusColumn: [string, (r: Item) => ReactNode] = [
    "Status",
    (r) => badge(r.state),
  ];
  const peopleTab =
    route.tab === "overview"
      ? all("membership.read")
        ? "members"
        : "teams"
      : route.tab;
  const settingsTab = route.tab === "overview" ? "general" : route.tab;
  const detail = route.page === "space" ? space : app;
  const label =
    route.page === "apps"
      ? "All Apps"
      : route.page === "space"
        ? space.display_name || "Space"
        : route.page === "app"
          ? app.display_name || "App"
          : pages.find((p) => p.id === route.page)?.label || "Home";
  const menu = pages
    .filter((p) => p.id !== "activity" || can("audit.read"))
    .filter(
      (p) =>
        p.id !== "people" ||
        all("membership.read") ||
        can("team.manage") ||
        all("authorization.read"),
    );
  const tabs =
    route.page === "space"
      ? [
          ["overview", "Overview"],
          ["apps", "Apps"],
          ["database", "Database"],
          ...(scoped("authorization.read") ? [["access", "Access"]] : []),
          ["settings", "Settings"],
        ]
      : route.page === "app"
        ? [
            ["overview", "Overview"],
            ["microapps", "Micro-apps"],
            ...(scoped("authorization.read") ? [["access", "Access"]] : []),
            ["settings", "Settings"],
          ]
        : route.page === "people"
          ? [
              ...(all("membership.read") ? [["members", "Members"]] : []),
              ...(can("team.read") ? [["teams", "Teams"]] : []),
              ...(all("authorization.read") ? [["roles", "Tenant roles"]] : []),
            ]
          : route.page === "settings"
            ? [
                ["general", "General"],
                ...(all("database.read") ? [["databases", "Databases"]] : []),
                ["account", "My account"],
              ]
            : [];
  const activeTab =
    route.page === "people"
      ? peopleTab
      : route.page === "settings"
        ? settingsTab
        : route.tab;
  return (
    <div
      className={
        "shell tenant-console" +
        (collapsed ? " sidebar-collapsed" : "") +
        (navOpen ? " navigation-open" : "")
      }
    >
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      {mobile && navOpen && (
        <button
          className="navigation-backdrop"
          aria-label="Close navigation"
          onClick={() => setNavOpen(false)}
        />
      )}
      <aside
        className="sidebar"
        id="tenant-navigation"
        ref={navRef as any}
        inert={mobile && !navOpen}
        role={mobile && navOpen ? "dialog" : undefined}
        aria-modal={mobile && navOpen ? true : undefined}
        aria-label="Tenant navigation"
      >
        <div className="sidebar-brand">
          <span className="brand-mark">
            <Layers3 size={23} />
          </span>
          <strong>
            lastapp<span>Tenant workspace</span>
          </strong>
          {mobile && (
            <button
              className="icon"
              aria-label="Close navigation"
              onClick={() => setNavOpen(false)}
            >
              <X size={20} />
            </button>
          )}
        </div>
        <div className="tenant-switch">
          <span className="nav-label">COMPANY</span>
          {memberships.length > 1 || me.pagination?.has_more ? (
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
          )}
        </div>
        <nav aria-label="Console screens">
          <div className="nav-group">
            {menu.map((p) => {
              const Icon = p.icon;
              const selected =
                p.id === route.page ||
                (p.id === "home" && route.page === "apps") ||
                (p.id === "spaces" && ["space", "app"].includes(route.page));
              return (
                <button
                  key={p.id}
                  className={selected ? "selected" : ""}
                  aria-current={selected ? "page" : undefined}
                  title={p.label}
                  onClick={() => go(p.id)}
                >
                  <Icon size={19} />
                  <span>{p.label}</span>
                  {selected && (
                    <ChevronRight size={14} className="nav-chevron" />
                  )}
                </button>
              );
            })}
          </div>
        </nav>
        <div className="sidebar-bottom">
          <span className="env">
            <i />
            {profile.plan_name || "Your workspace"}
          </span>
          <div className="sidebar-version">Tenant → Space → App</div>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="topbar-left">
            <button
              className="icon menu-toggle"
              aria-label={
                mobile
                  ? "Open navigation"
                  : collapsed
                    ? "Expand sidebar"
                    : "Collapse sidebar"
              }
              aria-controls="tenant-navigation"
              aria-expanded={mobile ? navOpen : !collapsed}
              onClick={() =>
                mobile ? setNavOpen(true) : setCollapsed(!collapsed)
              }
            >
              <Menu size={21} />
            </button>
            <span className="topbar-workspace">
              {profile.display_name || "Tenant workspace"}
            </span>
          </div>
          <div className="topbar-actions">
            <button
              className="icon mode-toggle"
              aria-label={
                isDark ? "Switch to light mode" : "Switch to dark mode"
              }
              onClick={() =>
                setAppearance({
                  ...appearance,
                  mode: isDark ? "light" : "dark",
                })
              }
            >
              {isDark ? <Sun size={20} /> : <Moon size={20} />}
            </button>
            <button
              className="icon appearance-trigger"
              aria-label="Customize appearance"
              onClick={() => setAppearanceOpen(true)}
            >
              <Palette size={20} />
            </button>
            <button
              className="profile-button"
              onClick={() => go("settings", { tab: "account" })}
            >
              <span className="avatar person">
                {me.display_name?.slice(0, 1)}
              </span>
              <span>
                <strong>{me.display_name}</strong>
                <small>
                  {all("tenant.manage")
                    ? "Tenant administrator"
                    : "Workspace member"}
                </small>
              </span>
            </button>
            <button className="icon" aria-label="Sign out" onClick={signOut}>
              <LogOut size={17} />
            </button>
          </div>
        </header>
        <section className="content" id="main-content" tabIndex={-1}>
          <div className="page-title-row">
            <h1>{label}</h1>
            <div className="breadcrumbs">
              <button className="text-button" onClick={() => go("home")}>
                {profile.display_name || "Company"}
              </button>
              {["space", "app"].includes(route.page) && (
                <>
                  <ChevronRight size={13} />
                  <button
                    className="text-button"
                    disabled={!scoped("space.read", route.space, "")}
                    onClick={() => openSpace({ id: route.space })}
                  >
                    {route.page === "space"
                      ? space.display_name
                      : app.space_name}
                  </button>
                </>
              )}
              {route.page === "app" && (
                <>
                  <ChevronRight size={13} />
                  <span>{app.display_name}</span>
                </>
              )}
            </div>
          </div>
          <div className="page-heading">
            <div>
              <h2>
                {route.page === "home"
                  ? `Welcome, ${me.display_name.split(" ")[0]}`
                  : label}
              </h2>
              <p>
                {route.page === "home"
                  ? "Build your company's workspace, one Space at a time."
                  : route.page === "space"
                    ? "Shared business data and rules for your Apps."
                    : route.page === "app"
                      ? "Client experiences within this Space."
                      : route.page === "people"
                        ? "Members, teams and their scoped roles."
                        : route.page === "settings"
                          ? "Company, database and account settings."
                          : route.page === "apps"
                            ? "Apps available to you across your company's Spaces."
                            : "Your workspace activity and resources."}
              </p>
            </div>
            <div className="row-actions">
              <button className="icon" aria-label="Refresh" onClick={refresh}>
                <RefreshCw size={16} />
              </button>
              {tenant &&
                ["home", "spaces"].includes(route.page) &&
                all("space.manage") && (
                  <button className="primary" onClick={createSpace}>
                    <Plus size={17} />
                    Create Space
                  </button>
                )}
              {route.page === "space" &&
                route.tab === "apps" &&
                scoped("app.manage") &&
                space.id && (
                  <button className="primary" onClick={createApp}>
                    <Plus size={17} />
                    Create App
                  </button>
                )}
              {route.page === "app" &&
                route.tab === "microapps" &&
                scoped("microapp.manage") &&
                app.id && (
                  <button className="primary" onClick={createMicro}>
                    <Plus size={17} />
                    Register micro-app
                  </button>
                )}
              {route.page === "people" &&
                peopleTab === "members" &&
                all("membership.manage") && (
                  <button className="primary" onClick={addMember}>
                    <Plus size={17} />
                    Add member
                  </button>
                )}
              {route.page === "settings" &&
                settingsTab === "databases" &&
                all("database.manage") && (
                  <button className="primary" onClick={addDatabase}>
                    <Plus size={17} />
                    Add database
                  </button>
                )}
            </div>
          </div>
          {error && (
            <div className="error" role="alert">
              {error}
              <button
                className="tiny"
                aria-label="Dismiss error"
                onClick={() => setError("")}
              >
                <X size={14} />
              </button>
            </div>
          )}
          {!!tabs.length && (
            <div className="tabs workspace-tabs">
              {tabs.map(([id, title]) => (
                <button
                  key={id}
                  className={activeTab === id ? "active" : ""}
                  onClick={() => {
                    setRoute((r) => ({ ...r, tab: id }));
                    setModal(null);
                  }}
                >
                  {title}
                </button>
              ))}
            </div>
          )}
          {!tenant ? (
            <div className="panel empty">
              <Building2 size={32} />
              <h3>No Tenant access yet</h3>
              <p>Your administrator can add this account to a Tenant.</p>
            </div>
          ) : (
            <>
              {route.page === "home" && (
                <>
                  {overview.spaces === 0 && (
                    <section className="panel first-space">
                      <span className="welcome-symbol">
                        <FolderOpen size={27} />
                      </span>
                      <div>
                        <h2>
                          {all("space.manage")
                            ? "Create your first Space"
                            : "Your workspace is ready"}
                        </h2>
                        <p>
                          {all("space.manage")
                            ? "Choose a database and give your Apps a shared foundation."
                            : "Your administrator can grant access to Spaces and Apps."}
                        </p>
                      </div>
                      {all("space.manage") && (
                        <button className="primary" onClick={createSpace}>
                          Create your first Space
                        </button>
                      )}
                    </section>
                  )}
                  <div className="tenant-stats">
                    {[
                      [
                        "Spaces",
                        overview.spaces,
                        profile.max_spaces,
                        FolderOpen,
                      ],
                      ["Apps", overview.apps, profile.max_apps, AppWindow],
                      [
                        "Databases",
                        overview.databases,
                        profile.max_databases,
                        Database,
                      ],
                    ]
                      .filter(([, count]) => count !== undefined)
                      .map(([title, count, limit, Icon]: any) => (
                        <section className="panel tenant-stat" key={title}>
                          <Icon size={23} />
                          <span>{title}</span>
                          <strong>{count ?? 0}</strong>
                          <small>
                            {limit == null
                              ? "No plan limit"
                              : `${limit} allowed by your plan`}
                          </small>
                        </section>
                      ))}
                  </div>
                  <section className="panel workspace-section">
                    <h3>Recent Apps</h3>
                    {can("app.read") && (
                      <button className="secondary" onClick={() => go("apps")}>
                        View all Apps <ChevronRight size={16} />
                      </button>
                    )}
                    {overview.recent_apps?.length ? (
                      <div className="app-grid">
                        {overview.recent_apps.map((a: Item) => (
                          <button
                            className="app-card"
                            key={a.id}
                            onClick={() => openApp(a)}
                          >
                            <AppWindow size={25} />
                            <strong>{a.display_name}</strong>
                            <small>{a.space_name}</small>
                            {badge(a.state)}
                          </button>
                        ))}
                      </div>
                    ) : (
                      <p className="muted">
                        Your Apps will appear here as you build them.
                      </p>
                    )}
                  </section>
                  {!!overview.attention?.length && (
                    <section className="panel workspace-section">
                      <h3>Needs attention</h3>
                      {overview.attention.map((d: Item) => (
                        <button
                          className="attention-row"
                          key={d.id}
                          onClick={() => go("settings", { tab: "databases" })}
                        >
                          <Database size={18} />
                          <span>
                            {d.database_name}
                            <small>
                              {d.last_error_message ||
                                "Database provisioning is in progress."}
                            </small>
                          </span>
                          {badge(d.state)}
                        </button>
                      ))}
                    </section>
                  )}
                </>
              )}
              {route.page === "spaces" && (
                <Collection
                  key={base + "spaces"}
                  path={base + "/spaces"}
                  reload={reload}
                  empty="Create a Space to start organizing your Apps."
                  columns={[
                    nameColumn,
                    statusColumn,
                    [
                      "Database",
                      (r) => (
                        <>
                          {r.database_name}
                          <small>
                            {r.database_layout} · {r.database_state}
                          </small>
                        </>
                      ),
                    ],
                    ["Apps", (r) => r.app_count],
                  ]}
                  actions={(r) => (
                    <button
                      className="text-button"
                      onClick={() => openSpace(r)}
                    >
                      Open <ChevronRight size={14} />
                    </button>
                  )}
                />
              )}
              {route.page === "apps" && (
                <Collection
                  key={base + "apps"}
                  path={base + "/apps"}
                  reload={reload}
                  empty="No Apps are available to this account."
                  columns={[
                    nameColumn,
                    ["Space", (r) => r.space_name],
                    statusColumn,
                  ]}
                  actions={(r) => (
                    <button className="text-button" onClick={() => openApp(r)}>
                      Open <ChevronRight size={14} />
                    </button>
                  )}
                />
              )}
              {route.page === "space" && space.id && (
                <>
                  {route.tab === "overview" && (
                    <section className="panel workspace-section">
                      <h3>{space.display_name}</h3>
                      <p>This Space is the shared foundation for its Apps.</p>
                      <dl className="workspace-facts">
                        <dt>Status</dt>
                        <dd>{badge(space.state)}</dd>
                        <dt>Database</dt>
                        <dd>
                          {space.database_name} · {space.database_layout} ·{" "}
                          {space.database_state}
                        </dd>
                      </dl>
                      <button
                        className="primary"
                        onClick={() => setRoute((r) => ({ ...r, tab: "apps" }))}
                      >
                        View Apps <ChevronRight size={16} />
                      </button>
                    </section>
                  )}
                  {route.tab === "apps" && (
                    <Collection
                      path={base + `/apps?space_id=${space.id}`}
                      reload={reload}
                      columns={[
                        nameColumn,
                        statusColumn,
                        ["Micro-apps", (r) => r.microapp_count],
                      ]}
                      empty="Create an App in this Space."
                      actions={(r) => (
                        <button
                          className="text-button"
                          onClick={() => openApp(r)}
                        >
                          Open <ChevronRight size={14} />
                        </button>
                      )}
                    />
                  )}
                  {route.tab === "database" && (
                    <section className="panel workspace-section">
                      <h3>Database binding</h3>
                      <dl className="workspace-facts">
                        <dt>Database</dt>
                        <dd>{space.database_name}</dd>
                        <dt>Layout</dt>
                        <dd>{space.database_layout}</dd>
                        <dt>Status</dt>
                        <dd>{badge(space.database_state)}</dd>
                      </dl>
                      {all("database.read") && (
                        <button
                          className="secondary"
                          onClick={() => go("settings", { tab: "databases" })}
                        >
                          Manage database resources
                        </button>
                      )}
                    </section>
                  )}
                  {route.tab === "access" && (
                    <AccessControl
                      tenant={tenant}
                      platform={false}
                      caps={caps}
                      refresh={refresh}
                      scopeKind="space"
                      scopeId={space.id}
                    />
                  )}
                  {route.tab === "settings" && (
                    <section className="panel workspace-section">
                      <h3>Space settings</h3>
                      <p>
                        {space.display_name} · {space.slug} · {space.state}
                      </p>
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
                    </section>
                  )}
                </>
              )}
              {route.page === "app" && app.id && (
                <>
                  {route.tab === "overview" && (
                    <section className="panel workspace-section">
                      <h3>{app.display_name}</h3>
                      <p>
                        Part of {app.space_name}. Its business definitions and
                        database come from that Space.
                      </p>
                      <dl className="workspace-facts">
                        <dt>Status</dt>
                        <dd>{badge(app.state)}</dd>
                        <dt>Compute preference</dt>
                        <dd>{app.compute_mode}</dd>
                      </dl>
                      <button
                        className="primary"
                        onClick={() =>
                          setRoute((r) => ({ ...r, tab: "microapps" }))
                        }
                      >
                        View micro-apps <ChevronRight size={16} />
                      </button>
                    </section>
                  )}
                  {route.tab === "microapps" && (
                    <Collection
                      path={base + `/microapps?app_id=${app.id}`}
                      reload={reload}
                      columns={[
                        nameColumn,
                        statusColumn,
                        ["Artifact", (r) => r.artifact_ref || "—"],
                        ["Order", (r) => r.navigation_order],
                      ]}
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
                                        label: "State",
                                        type: "select",
                                        options: states,
                                        required: true,
                                      },
                                      {
                                        key: "artifact_ref",
                                        label: "Artifact reference",
                                      },
                                      {
                                        key: "navigation_order",
                                        label: "Navigation order",
                                        type: "number",
                                      },
                                    ],
                                  )
                                }
                              >
                                Edit
                              </button>
                            )
                          : undefined
                      }
                    />
                  )}
                  {route.tab === "access" && (
                    <AccessControl
                      tenant={tenant}
                      platform={false}
                      caps={caps}
                      refresh={refresh}
                      scopeKind="app"
                      scopeId={app.id}
                    />
                  )}
                  {route.tab === "settings" && (
                    <section className="panel workspace-section">
                      <h3>App settings</h3>
                      <p>
                        {app.display_name} · {app.slug} · {app.state}
                      </p>
                      {scoped("app.manage") && (
                        <button
                          className="secondary"
                          onClick={() =>
                            edit(app, base + "/apps/" + app.id, "Edit App", [
                              {
                                key: "display_name",
                                label: "Name",
                                required: true,
                              },
                              {
                                key: "state",
                                label: "State",
                                type: "select",
                                options: states,
                                required: true,
                              },
                              {
                                key: "compute_mode",
                                label: "Compute preference",
                                type: "select",
                                options: [
                                  { value: "shared", label: "Shared" },
                                  ...(profile.allow_dedicated_compute ||
                                  app.compute_mode === "dedicated"
                                    ? [
                                        {
                                          value: "dedicated",
                                          label: "Dedicated",
                                        },
                                      ]
                                    : []),
                                ],
                                required: true,
                              },
                            ])
                          }
                        >
                          Edit App
                        </button>
                      )}
                    </section>
                  )}
                </>
              )}
              {route.page === "people" && peopleTab === "members" && (
                <Collection
                  path={base + "/memberships"}
                  reload={reload}
                  columns={[
                    [
                      "Member",
                      (r) => (
                        <>
                          <strong>{r.display_name}</strong>
                          <small>{r.email}</small>
                        </>
                      ),
                    ],
                    statusColumn,
                    ["Account", (r) => r.user_state],
                  ]}
                  actions={
                    all("membership.manage")
                      ? (r) => (
                          <button
                            className="text-button"
                            onClick={() =>
                              act(() =>
                                api(
                                  base + "/memberships/" + r.user_id,
                                  "PATCH",
                                  {
                                    state:
                                      r.state === "active"
                                        ? "disabled"
                                        : "active",
                                  },
                                  r.revision,
                                ),
                              )
                            }
                          >
                            {r.state === "active" ? "Disable" : "Enable"}
                          </button>
                        )
                      : undefined
                  }
                />
              )}
              {route.page === "people" && peopleTab === "teams" && (
                <AccessControl
                  key={tenant + "teams"}
                  tenant={tenant}
                  platform={false}
                  caps={caps}
                  refresh={refresh}
                  scopeKind="team"
                />
              )}
              {route.page === "people" && peopleTab === "roles" && (
                <AccessControl
                  key={tenant + "tenant"}
                  tenant={tenant}
                  platform={false}
                  caps={caps}
                  refresh={refresh}
                  scopeKind="tenant"
                />
              )}
              {route.page === "activity" && (
                <Collection
                  path={base + "/audit-events"}
                  reload={reload}
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
              {route.page === "settings" && settingsTab === "general" && (
                <section className="panel workspace-section">
                  <h3>Company and subscription</h3>
                  <dl className="workspace-facts">
                    <dt>Company</dt>
                    <dd>{profile.display_name}</dd>
                    <dt>Status</dt>
                    <dd>{badge(profile.state || "active")}</dd>
                    <dt>Plan</dt>
                    <dd>{profile.plan_name}</dd>
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
                        edit(profile, base, "Edit company", [
                          {
                            key: "display_name",
                            label: "Company name",
                            required: true,
                          },
                        ])
                      }
                    >
                      Edit company
                    </button>
                  )}
                  <p className="muted">
                    Your platform administrator manages the subscription plan.
                  </p>
                </section>
              )}
              {route.page === "settings" && settingsTab === "databases" && (
                <Collection
                  path={base + "/database-resources"}
                  reload={reload}
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
                                onClick={() =>
                                  act(() =>
                                    api(
                                      base +
                                        `/database-resources/${r.id}/retry`,
                                      "POST",
                                      {},
                                      r.revision,
                                    ),
                                  )
                                }
                              >
                                Verify / retry
                              </button>
                            )}
                            {r.management_mode === "registered" &&
                              r.state !== "provisioning" &&
                              r.state !== "retired" && (
                                <button
                                  className="text-button"
                                  onClick={() =>
                                    setModal({
                                      title: "Update database credentials",
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
                            {r.state !== "provisioning" &&
                              r.state !== "retired" && (
                                <button
                                  className="text-button danger"
                                  onClick={() => {
                                    if (
                                      window.confirm(
                                        "Retire this database resource? Physical data is preserved.",
                                      )
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
          {route.page === "settings" && settingsTab === "account" && (
            <>
              <section className="panel workspace-section">
                <h3>My account</h3>
                <p>
                  {me.display_name} · {me.email}
                </p>
                <small>User ID: {me.id}</small>
              </section>
              {me.auth_method === "password" ? (
                <PasswordScreen
                  done={async () => {
                    await refreshMe();
                    setToast("Password changed successfully");
                  }}
                />
              ) : (
                <section className="panel workspace-section">
                  <p>
                    You use single sign-on. Manage your password with your
                    identity provider.
                  </p>
                </section>
              )}
            </>
          )}
          <footer className="page-footer">
            <span>© {new Date().getFullYear()} Lastapp</span>
            <span>Tenant → Space → App → micro-app</span>
          </footer>
        </section>
      </main>
      {appearanceOpen && (
        <AppearancePanel
          value={appearance}
          onChange={setAppearance}
          reset={reset}
          close={() => setAppearanceOpen(false)}
        />
      )}
      {modal && (
        <FormModal
          key={tenant + modal.title}
          modal={modal}
          lookups={{}}
          close={() => setModal(null)}
          done={done}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Check size={17} />
          {toast}
        </div>
      )}
    </div>
  );
}
