import { FormModal, type Field, type Modal } from "./form-modal";
import { LoginScreen, PasswordScreen } from "./login";
import { TenantWorkspace } from "./tenant-workspace";
import { AccessControl } from "./access-control";
import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Layers3,
  LayoutDashboard,
  Building2,
  Database,
  Boxes,
  AppWindow,
  PanelsTopLeft,
  Users,
  ShieldCheck,
  ScrollText,
  Server,
  SlidersHorizontal,
  Plus,
  Search,
  ChevronRight,
  ArrowUpRight,
  RefreshCw,
  LogOut,
  X,
  Check,
  LoaderCircle,
  MoreHorizontal,
  FolderOpen,
  KeyRound,
  Menu,
  Palette,
  Moon,
  Sun,
  ChevronDown,
  PanelLeftClose,
} from "lucide-react";
import "./style.css";
import { api, resetSessionRequests, sessionExpiredEvent } from "./api";
import { RemoteSelect } from "./remote-select";
import { AppearancePanel, useAppearance, useDialogFocus } from "./appearance";
import { OverviewDashboard } from "./dashboard";
import { NavigationSearch } from "./navigation-search";
import { readPreference, savePreference } from "./preferences";
type Item = Record<string, any>;
const text = (v: any) => (v === null || v === undefined ? "—" : String(v));
const short = (v: any) => String(v || "").slice(0, 8);
const stateBadge = (v: any) => (
  <span className={"badge " + v}>
    <i />
    {text(v).replaceAll("_", " ")}
  </span>
);
const choose = (items: Item[], label = "display_name", value = "id") =>
  items.map((x) => ({ value: x[value], label: x[label] }));
const states = ["active", "suspended", "archived"].map((value) => ({
  value,
  label: value,
}));
const membershipStates = ["active", "disabled"].map((value) => ({
  value,
  label: value,
}));
const initialId = () => ({ id: crypto.randomUUID() });
const nameFields: Field[] = [
  { key: "display_name", label: "Name", required: true },
  {
    key: "slug",
    label: "Slug",
    required: true,
    hint: "Lowercase letters, numbers and hyphens. Immutable after creation.",
  },
];
const pages: Item[] = [
  { id: "overview", label: "Overview", icon: LayoutDashboard, platform: true },
  { id: "tenants", label: "Tenants", icon: Building2, platform: true },
  {
    id: "tenant-detail",
    label: "Tenant details",
    icon: Building2,
    hidden: true,
  },
  { id: "spaces", label: "Spaces", icon: FolderOpen, permission: "space.read" },
  { id: "apps", label: "Apps", icon: AppWindow, permission: "app.read" },
  {
    id: "microapps",
    label: "Micro-apps",
    icon: PanelsTopLeft,
    permission: "microapp.read",
  },
  {
    id: "databases",
    label: "Databases",
    icon: Database,
    permission: "database.read",
  },
  {
    id: "memberships",
    label: "Members",
    icon: Users,
    permission: "membership.read",
  },
  {
    id: "access",
    label: "Access control",
    icon: ShieldCheck,
    permission: "authorization.read",
  },
  { id: "targets", label: "Database targets", icon: Server, platform: true },
  { id: "plans", label: "Plans", icon: SlidersHorizontal, platform: true },
  { id: "users", label: "Users", icon: Users, platform: true },
  {
    id: "audit",
    label: "Activity",
    icon: ScrollText,
    permission: "audit.read",
  },
];
function App() {
  const {
    appearance,
    setAppearance,
    isDark,
    reset: resetAppearance,
  } = useAppearance();
  const [appearanceOpen, setAppearanceOpen] = useState(false);
  const [navigationSearch, setNavigationSearch] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [mobile, setMobile] = useState(
    () => matchMedia("(max-width: 800px)").matches,
  );
  const sidebarRef = useDialogFocus(
    () => setNavigationOpen(false),
    mobile && navigationOpen,
  );
  useEffect(() => {
    const media = matchMedia("(max-width: 800px)");
    const resize = () => {
      setMobile(media.matches);
      setNavigationOpen(false);
    };
    media.addEventListener("change", resize);
    return () => media.removeEventListener("change", resize);
  }, []);
  const [me, setMe] = useState<Item | null>(null),
    [auth, setAuth] = useState<"loading" | "in" | "out" | "change-password">(
      "loading",
    );
  const [tenants, setTenants] = useState<Item[]>([]),
    [plans, setPlans] = useState<Item[]>([]),
    [users, setUsers] = useState<Item[]>([]);
  const [tenant, setTenant] = useState(() => readPreference("lastapp-tenant"));
  const [page, setPage] = useState(() =>
    readPreference("lastapp-page", "overview"),
  );
  const [caps, setCaps] = useState<Item>({});
  const [collection, setCollection] = useState<{ key: string; items: Item[] }>({
      key: "",
      items: [],
    }),
    [overview, setOverview] = useState<Item>({}),
    [support, setSupport] = useState<Item>({});
  const [loading, setLoading] = useState(true),
    [error, setError] = useState(""),
    [toast, setToast] = useState(""),
    [search, setSearch] = useState(""),
    [tab, setTab] = useState("grants");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setQuery(search), 250);
    return () => clearTimeout(timer);
  }, [search]);
  const [recordPage, setRecordPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [tenantSearch, setTenantSearch] = useState("");
  const [tenantPageNumber, setTenantPageNumber] = useState(1);
  const [moreTenants, setMoreTenants] = useState(false);
  const [modal, setModal] = useState<Modal | null>(null),
    [reload, setReload] = useState(0);
  function endSession() {
    resetSessionRequests();
    setAuth("out");
    setMe(null);
    setTenant("");
    setCaps({});
    setModal(null);
    setCollection({ key: "", items: [] });
    setOverview({});
    setSupport({});
    setTenants([]);
    setPlans([]);
    setUsers([]);
    setError("");
    setToast("");
    setPage("overview");
    savePreference("lastapp-tenant", "");
    savePreference("lastapp-page", "overview");
  }
  useEffect(() => {
    window.addEventListener(sessionExpiredEvent, endSession);
    return () => window.removeEventListener(sessionExpiredEvent, endSession);
  }, []);
  const collectionKey =
    page + ":" + tenant + ":" + tab + ":" + recordPage + ":" + query;
  const data = collection.key === collectionKey ? collection.items : [];
  function setData(items: Item[]) {
    setCollection({ key: collectionKey, items });
  }
  const base = `/v1/tenants/${tenant}`;
  const currentTenant = tenants.find((t) => t.id === tenant);
  const platform = !!me?.platform_admin;
  const tenantPage =
    [
      "spaces",
      "apps",
      "microapps",
      "databases",
      "memberships",
      "tenant-detail",
    ].includes(page) ||
    (["access", "audit"].includes(page) && !!tenant);
  const can = (permission: string) => platform || !!caps[permission]?.available;
  const all = (permission: string) => platform || !!caps[permission]?.all;
  function go(id: string) {
    setRecordPage(1);
    setNavigationOpen(false);
    setNavigationSearch(false);
    setData([]);
    setPage(id);
    savePreference("lastapp-page", id);
    setSearch("");
    setError("");
  }
  function selectTenant(id: string) {
    setModal(null);
    setRecordPage(1);
    setData([]);
    setTenant(id);
    savePreference("lastapp-tenant", id);
    if (
      id &&
      ["overview", "tenants", "targets", "plans", "users"].includes(page)
    )
      go(platform ? "tenant-detail" : "spaces");
    if (!id && tenantPage) go(platform ? "overview" : "spaces");
  }
  useEffect(() => {
    if (auth !== "in" || !platform) return;
    let active = true;
    const timer = setTimeout(
      () =>
        api(
          (platform ? "/admin/v1/tenants" : "/v1/my-tenants") +
            `?page=${tenantPageNumber}&q=${encodeURIComponent(tenantSearch)}`,
        )
          .then((v) => {
            if (active) {
              setTenants(v.items);
              setMoreTenants(!!v.pagination?.has_more);
            }
          })
          .catch((e) => {
            if (active) setError(e.message);
          }),
      200,
    );
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [auth, platform, tenantSearch, tenantPageNumber]);
  async function loadGlobals() {
    const who = await api("/v1/me");
    setMe(who);
    if (who.password_change_required) {
      setAuth("change-password");
      return;
    }
    setAuth("in");
    if (!who.platform_admin) return;
    if (
      ![
        "overview",
        "tenants",
        "targets",
        "plans",
        "users",
        "access",
        "audit",
        "tenant-detail",
      ].includes(page)
    )
      go("overview");
    const [ts, ps, us] = await Promise.all([
      api(
        (who.platform_admin ? "/admin/v1/tenants" : "/v1/my-tenants") +
          `?page=${tenantPageNumber}&q=${encodeURIComponent(tenantSearch)}`,
      ),
      who.platform_admin
        ? api("/admin/v1/plans")
        : Promise.resolve({ items: [] }),
      who.platform_admin
        ? api("/admin/v1/users")
        : Promise.resolve({ items: [] }),
    ]);
    setTenants(ts.items);
    setMoreTenants(!!ts.pagination?.has_more);
    setPlans(ps.items);
    setUsers(us.items);
    if (!who.platform_admin && auth === "loading") go("spaces");
    if (!who.platform_admin && !tenant && ts.items.length) {
      setTenant(ts.items[0].id);
      go("spaces");
    }
  }
  useEffect(() => {
    loadGlobals().catch((e) => {
      if (e.status === 401) setAuth("out");
      else {
        setError(e.message);
        setAuth("out");
      }
    });
  }, []);
  useEffect(() => {
    if (auth !== "in" || !tenant) {
      setCaps({});
      return;
    }
    let active = true;
    api(base + "/capabilities")
      .then((v) => {
        if (active) setCaps(v.permissions);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [auth, tenant, reload]);
  useEffect(() => {
    if (auth !== "in" || !platform) return;
    let active = true;
    setLoading(true);
    setError("");
    setData([]);
    async function load() {
      if (tenantPage && !tenant) {
        setLoading(false);
        return;
      }
      let path = "";
      const aux: Item = {};
      switch (page) {
        case "overview": {
          const v = await api("/admin/v1/overview");
          if (active) setOverview(v);
          break;
        }
        case "tenant-detail": {
          const v = await api(base + "/overview");
          if (active) setOverview(v);
          break;
        }
        case "tenants":
          path = "/admin/v1/tenants";
          break;
        case "targets":
          path = "/admin/v1/database-targets";
          break;
        case "plans":
          path = "/admin/v1/plans";
          break;
        case "users":
          path = "/admin/v1/users";
          break;
        case "spaces":
          path = base + "/spaces";
          break;
        case "apps":
          path = base + "/apps";
          if (can("space.read"))
            aux.spaces = (await api(base + "/spaces")).items;
          break;
        case "microapps":
          path = base + "/microapps";
          if (can("app.read")) aux.apps = (await api(base + "/apps")).items;
          break;
        case "databases":
          path = base + "/database-resources";
          break;
        case "memberships":
          path = base + "/memberships";
          break;
        case "access":
          break;
        case "audit":
          path = tenant ? base + "/audit-events" : "/admin/v1/audit-events";
          break;
      }
      if (path) {
        const v = await api(
          path + `?page=${recordPage}&q=${encodeURIComponent(query)}`,
        );
        if (active) {
          setData(v.items);
          setHasMore(!!v.pagination?.has_more);
        }
      }
      if (active) {
        setSupport(aux);
        setLoading(false);
      }
    }
    load().catch((e) => {
      if (active) {
        setError(e.message);
        setLoading(false);
        if (e.status === 401) setAuth("out");
      }
    });
    return () => {
      active = false;
    };
  }, [auth, page, tenant, tab, reload, recordPage, query]);
  useEffect(() => {
    if (
      auth !== "in" ||
      !["databases", "spaces", "overview"].includes(page) ||
      modal
    )
      return;
    const timer = setInterval(() => setReload((v) => v + 1), 5000);
    return () => clearInterval(timer);
  }, [auth, page, modal]);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 4500);
    return () => clearTimeout(timer);
  }, [toast]);
  function completed() {
    setModal(null);
    setReload((v) => v + 1);
    loadGlobals().catch((e) => setError(e.message));
    setToast("Changes saved successfully");
  }
  async function action(fn: () => Promise<any>) {
    try {
      await fn();
      completed();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  function edit(
    entity: Item,
    resource: string,
    title: string,
    fields?: Field[],
  ) {
    const initial: Item = {
      display_name: entity.display_name,
      state: entity.state,
    };
    setModal({
      title,
      initial,
      fields: fields || [
        { key: "display_name", label: "Name", required: true },
        {
          key: "state",
          label: "State",
          type: "select",
          options: states,
          required: true,
        },
      ],
      save: (v) =>
        api(
          resource,
          "PATCH",
          Object.fromEntries(
            Object.entries(v).filter(([key, value]) => value !== entity[key]),
          ),
          entity.revision,
        ),
    });
  }
  async function create() {
    try {
      const init = initialId();
      switch (page) {
        case "tenants":
          setModal({
            title: "Create a Tenant",
            description: "A company workspace, managed centrally.",
            initial: {
              ...init,
              plan_id: plans[0]?.id,
              account_mode: "new",
              initial_admin_user_id: users.find(
                (u) => u.oidc_subject === "tenant-admin",
              )?.id,
            },
            fields: [
              ...nameFields,
              {
                key: "plan_id",
                label: "Plan",
                type: "select",
                options: choose(plans),
                required: true,
              },
              {
                key: "account_mode",
                label: "Administrator account",
                type: "select",
                required: true,
                options: [
                  { value: "new", label: "Create an email/password account" },
                  { value: "existing", label: "Use an existing account" },
                ],
              },
              {
                key: "admin_name",
                label: "Administrator name",
                required: true,
                when: (v) => v.account_mode === "new",
              },
              {
                key: "admin_email",
                label: "Administrator email",
                type: "email",
                required: true,
                when: (v) => v.account_mode === "new",
              },
              {
                key: "admin_password",
                label: "Initial password",
                type: "password",
                required: true,
                generate: true,
                hint: "15–128 characters. The administrator must change it at first login.",
                when: (v) => v.account_mode === "new",
              },
              {
                key: "initial_admin_user_id",
                label: "Initial Tenant administrator",
                type: "select",
                options: choose(users.filter((u) => u.state === "active")),
                required: true,
                when: (v) => v.account_mode === "existing",
              },
            ],
            save: (v) =>
              api("/admin/v1/tenants", "POST", {
                id: v.id,
                slug: v.slug,
                display_name: v.display_name,
                plan_id: v.plan_id,
                ...(v.account_mode === "existing"
                  ? { initial_admin_user_id: v.initial_admin_user_id }
                  : {
                      initial_admin: {
                        display_name: v.admin_name,
                        email: v.admin_email,
                        password: v.admin_password,
                      },
                    }),
              }),
          });
          break;
        case "targets":
          setModal({
            title: "Add a database target",
            description:
              "The server endpoint and credentials used for managed provisioning.",
            initial: {
              ...init,
              port: 3306,
              tls_mode: "verify_identity",
              managed_enabled: true,
            },
            fields: [
              { key: "display_name", label: "Name", required: true },
              { key: "canonical_host", label: "Hostname / IP", required: true },
              { key: "port", label: "Port", type: "number", required: true },
              {
                key: "tls_mode",
                label: "TLS",
                type: "select",
                options: [
                  { value: "verify_identity", label: "Verify server identity" },
                  {
                    value: "disabled",
                    label: "Disabled (local development only)",
                  },
                ],
                required: true,
              },
              { key: "username", label: "Provisioning username" },
              {
                key: "password",
                label: "Provisioning password",
                type: "password",
              },
              {
                key: "managed_enabled",
                label: "Allow managed creation",
                type: "checkbox",
              },
            ],
            save: (v) =>
              api("/admin/v1/database-targets", "POST", {
                id: v.id,
                display_name: v.display_name,
                canonical_host: v.canonical_host,
                port: Number(v.port),
                tls_mode: v.tls_mode,
                managed_enabled: v.managed_enabled,
                ...(v.username
                  ? {
                      credentials: {
                        username: v.username,
                        password: v.password,
                      },
                    }
                  : {}),
              }),
          });
          break;
        case "plans":
          planModal();
          break;
        case "users":
          setModal({
            title: "Create account",
            description:
              "Create a global email/password account. Tenant administrators can then add this person by email or user ID.",
            initial: {},
            fields: [
              { key: "display_name", label: "Name", required: true },
              { key: "email", label: "Email", type: "email", required: true },
              {
                key: "password",
                label: "Initial password",
                type: "password",
                required: true,
                generate: true,
                hint: "15–128 characters. The person must change it at first login.",
              },
            ],
            save: (v) => api("/admin/v1/users", "POST", v),
          });
          break;
      }
    } catch (e) {
      setError((e as Error).message);
    }
  }
  function planModal(plan?: Item) {
    setModal({
      title: plan ? "Edit plan" : "Create a plan",
      description:
        "Changes apply to future allocations; existing resources are preserved.",
      initial: plan
        ? {
            ...plan,
            allow_shared_database: !!plan.allow_shared_database,
            allow_dedicated_database: !!plan.allow_dedicated_database,
            allow_managed_database: !!plan.allow_managed_database,
            allow_registered_database: !!plan.allow_registered_database,
            allow_dedicated_compute: !!plan.allow_dedicated_compute,
          }
        : {
            ...initialId(),
            default_layout: "shared",
            allow_shared_database: true,
            allow_dedicated_database: true,
            allow_managed_database: true,
            allow_registered_database: true,
          },
      fields: [
        { key: "display_name", label: "Name", required: true },
        { key: "code", label: "Code", required: true },
        {
          key: "default_layout",
          label: "Default layout",
          type: "select",
          options: [
            { value: "shared", label: "Shared" },
            { value: "dedicated", label: "Dedicated" },
          ],
          required: true,
        },
        ...["spaces", "apps", "databases"].map((x) => ({
          key: "max_" + x,
          label: "Maximum " + x,
          type: "number",
          hint: "Blank means unlimited.",
        })),
        ...[
          "shared_database",
          "dedicated_database",
          "managed_database",
          "registered_database",
          "dedicated_compute",
        ].map((x) => ({
          key: "allow_" + x,
          label: "Allow " + x.replaceAll("_", " "),
          type: "checkbox",
        })),
      ],
      save: (v) => {
        const out: Item = {};
        for (const key of [
          "id",
          "code",
          "display_name",
          "default_layout",
          "allow_shared_database",
          "allow_dedicated_database",
          "allow_managed_database",
          "allow_registered_database",
          "allow_dedicated_compute",
        ])
          out[key] = v[key] ?? false;
        for (const k of ["max_spaces", "max_apps", "max_databases"])
          out[k] =
            v[k] === "" || v[k] === null || v[k] === undefined
              ? null
              : Number(v[k]);
        return api(
          "/admin/v1/plans" + (plan ? "/" + plan.id : ""),
          plan ? "PATCH" : "POST",
          out,
          plan?.revision,
        );
      },
    });
  }
  const managedPermission = (
    {
      spaces: "space.manage",
      apps: "app.manage",
      microapps: "microapp.manage",
      databases: "database.manage",
      memberships: "membership.manage",
      access: "authorization.manage",
    } as Item
  )[page];
  const editable = platform
    ? ["tenants", "targets", "plans", "users", "memberships"].includes(page)
    : managedPermission && can(managedPermission);
  const canOn = (permission: string, space: string, app: string) =>
    (platform &&
      ["tenants", "targets", "plans", "memberships"].includes(page)) ||
    all(permission) ||
    (caps[permission]?.spaces || []).includes(space) ||
    (caps[permission]?.apps || []).includes(app);
  const creatable = platform
    ? ["tenants", "targets", "plans", "users", "memberships"].includes(page)
    : managedPermission &&
      (all(managedPermission) ||
        (page === "apps" &&
          (caps[managedPermission]?.spaces || []).length > 0) ||
        (page === "microapps" && can(managedPermission)));
  const filtered = data;
  const visiblePages = pages.filter(
    (p) =>
      !p.hidden &&
      (platform
        ? p.platform || ["access", "audit"].includes(p.id)
        : !p.platform && (!p.permission || (tenant && can(p.permission)))),
  );
  const pageInfo = pages.find((p) => p.id === page) || pages[0];
  function rowActions(d: Item) {
    if (!editable) return null;
    if (
      tenantPage &&
      managedPermission &&
      !canOn(
        managedPermission,
        page === "spaces" ? d.id : d.space_id || "",
        page === "apps" ? d.id : d.app_id || "",
      )
    )
      return null;
    switch (page) {
      case "tenants":
        return (
          <>
            <button
              className="text-button"
              onClick={() => {
                selectTenant(d.id);
                go("spaces");
              }}
            >
              Open <ArrowUpRight size={13} />
            </button>
            <button
              className="icon"
              aria-label={"Edit " + d.display_name}
              onClick={() => {
                setModal({
                  title: "Edit Tenant",
                  initial: {
                    display_name: d.display_name,
                    state: d.state,
                    plan_id: d.plan_id,
                    default_layout: d.default_layout || "",
                  },
                  fields: [
                    { key: "display_name", label: "Name", required: true },
                    {
                      key: "state",
                      label: "State",
                      type: "select",
                      options: states,
                      required: true,
                    },
                    {
                      key: "plan_id",
                      label: "Plan",
                      type: "select",
                      options: choose(plans),
                      required: true,
                    },
                    {
                      key: "default_layout",
                      label: "Default layout override",
                      type: "select",
                      options: [
                        { value: "", label: "Use plan default" },
                        { value: "shared", label: "Shared" },
                        { value: "dedicated", label: "Dedicated" },
                      ],
                    },
                  ],
                  save: (v) =>
                    api("/admin/v1/tenants/" + d.id, "PATCH", v, d.revision),
                });
              }}
            >
              <MoreHorizontal size={18} />
            </button>
          </>
        );
      case "plans":
        return (
          <button className="text-button" onClick={() => planModal(d)}>
            Edit plan
          </button>
        );
      case "targets":
        return (
          <button
            className="text-button"
            onClick={() =>
              setModal({
                title: "Edit database target",
                description:
                  "Leave credentials empty to keep the current provisioning account.",
                initial: { display_name: d.display_name, state: d.state },
                fields: [
                  { key: "display_name", label: "Name", required: true },
                  {
                    key: "state",
                    label: "State",
                    type: "select",
                    options: membershipStates,
                    required: true,
                  },
                  { key: "username", label: "Provisioning username" },
                  {
                    key: "password",
                    label: "Provisioning password",
                    type: "password",
                  },
                ],
                save: (v) =>
                  api(
                    "/admin/v1/database-targets/" + d.id,
                    "PATCH",
                    {
                      display_name: v.display_name,
                      state: v.state,
                      ...(v.username
                        ? {
                            credentials: {
                              username: v.username,
                              password: v.password,
                            },
                          }
                        : {}),
                    },
                    d.revision,
                  ),
              })
            }
          >
            Manage
          </button>
        );
      case "users":
        return (
          <button
            className="text-button"
            onClick={() =>
              action(() =>
                api(
                  "/admin/v1/users/" + d.id,
                  "PATCH",
                  { state: d.state === "active" ? "disabled" : "active" },
                  d.revision,
                ),
              )
            }
          >
            {d.state === "active" ? "Disable" : "Enable"}
          </button>
        );
      default:
        return null;
    }
  }
  function columns(): [string, (d: Item) => React.ReactNode][] {
    switch (page) {
      case "tenants":
        return [
          [
            "Tenant",
            (d) => (
              <div className="entity">
                <span className="avatar">
                  {d.display_name.slice(0, 2).toUpperCase()}
                </span>
                <span>
                  <strong>{d.display_name}</strong>
                  <small>{d.slug}</small>
                </span>
              </div>
            ),
          ],
          ["Plan", (d) => d.plan_name],
          ["Spaces", (d) => d.space_count],
          ["Apps", (d) => d.app_count],
          ["Status", (d) => stateBadge(d.state)],
        ];
      case "targets":
        return [
          [
            "Target",
            (d) => (
              <>
                <strong>{d.display_name}</strong>
                <small>
                  {d.canonical_host}:{d.port}
                </small>
              </>
            ),
          ],
          [
            "TLS",
            (d) => (
              <span className="tag">{d.tls_mode.replaceAll("_", " ")}</span>
            ),
          ],
          [
            "Managed creation",
            (d) => (d.managed_enabled ? "Enabled" : "Disabled"),
          ],
          ["Status", (d) => stateBadge(d.state)],
        ];
      case "plans":
        return [
          [
            "Plan",
            (d) => (
              <>
                <strong>{d.display_name}</strong>
                <small>{d.code}</small>
              </>
            ),
          ],
          ["Default", (d) => d.default_layout],
          ["Spaces", (d) => d.max_spaces ?? "Unlimited"],
          ["Apps", (d) => d.max_apps ?? "Unlimited"],
          ["Databases", (d) => d.max_databases ?? "Unlimited"],
        ];
      case "users":
      case "memberships":
        return [
          [
            "User",
            (d) => (
              <div className="entity">
                <span className="avatar person">
                  {d.display_name.slice(0, 1)}
                </span>
                <span>
                  <strong>{d.display_name}</strong>
                  <small>{d.email || short(d.user_id || d.id)}</small>
                </span>
              </div>
            ),
          ],
          ["Status", (d) => stateBadge(d.state)],
          ["Registered", (d) => new Date(d.created_at).toLocaleDateString()],
        ];
      case "audit":
        return [
          [
            "Event",
            (d) => (
              <>
                <strong>{d.action.replaceAll(".", " · ")}</strong>
                <small>
                  {d.resource_type} · {short(d.resource_id)}
                </small>
              </>
            ),
          ],
          ["Actor", (d) => d.actor_name],
          ["Time", (d) => new Date(d.created_at).toLocaleString()],
          ["Request", (d) => <code>{short(d.request_id)}</code>],
        ];
      default:
        return [];
    }
  }
  if (auth === "loading")
    return (
      <div className="boot">
        <span className="brand-mark">
          <Layers3 />
        </span>
        <LoaderCircle className="spin" /> Opening Lastapp…
      </div>
    );
  function signOut() {
    api("/auth/logout", "POST", {})
      .then(endSession)
      .catch((e) => {
        if (e.status === 401) endSession();
        else setError(e.message);
      });
  }
  if (auth === "change-password")
    return <PasswordScreen required done={loadGlobals} signOut={signOut} />;
  if (auth === "out") return <LoginScreen done={loadGlobals} />;
  if (!platform)
    return (
      <TenantWorkspace
        me={me!}
        appearanceControls={{
          appearance,
          setAppearance,
          isDark,
          reset: resetAppearance,
        }}
        signOut={signOut}
        refreshMe={loadGlobals}
      />
    );
  const subtitle = (
    {
      overview: "A clear view of your platform, from Tenants to databases.",
      tenants: "The companies building on your platform.",
      spaces: "Business data and rules, organized into Spaces.",
      apps: "Backend API units serving your client micro-apps.",
      microapps: "Client experiences registered under your Apps.",
      databases: "Managed creation and existing database connections.",
      targets: "MariaDB endpoints available for provisioning.",
      plans: "Database layouts, provisioning options and resource limits.",
      users: "Platform identities from your identity provider.",
      memberships: "Who belongs to this Tenant. Access is assigned separately.",
      access:
        "Fixed administration roles, team relationships and scoped business roles.",
      audit: "A record of administrative changes and provisioning events.",
    } as Item
  )[page];
  return (
    <div
      className={
        "shell" +
        (sidebarCollapsed ? " sidebar-collapsed" : "") +
        (navigationOpen ? " navigation-open" : "")
      }
    >
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      {mobile && navigationOpen && (
        <div
          className="navigation-backdrop"
          onClick={() => setNavigationOpen(false)}
        />
      )}
      <aside
        className="sidebar"
        id="main-navigation"
        ref={sidebarRef}
        inert={mobile && !navigationOpen}
        role={mobile && navigationOpen ? "dialog" : undefined}
        aria-modal={mobile && navigationOpen ? true : undefined}
        aria-label="Main navigation"
        tabIndex={-1}
      >
        <div className="brand-row">
          <button
            className="brand"
            aria-label="Lastapp home"
            onClick={() => go(platform ? "overview" : "spaces")}
          >
            <span className="brand-mark">
              <Layers3 size={24} />
            </span>
            <b>
              lastapp<span>CONTROL PLANE</span>
            </b>
          </button>
          <button
            className="icon sidebar-toggle"
            aria-label={mobile ? "Close navigation" : "Collapse sidebar"}
            onClick={() =>
              mobile ? setNavigationOpen(false) : setSidebarCollapsed(true)
            }
          >
            {mobile ? <X size={18} /> : <PanelLeftClose size={17} />}
          </button>
        </div>
        <div className="context-picker">
          <label htmlFor="tenant-context">WORKSPACE</label>
          <input
            aria-label="Search Tenants"
            value={tenantSearch}
            placeholder="Search Tenants…"
            onChange={(e) => {
              setTenantSearch(e.target.value);
              setTenantPageNumber(1);
            }}
          />
          <div className="lookup-controls">
            <button
              disabled={tenantPageNumber === 1}
              onClick={() => setTenantPageNumber((p) => p - 1)}
              aria-label="Previous Tenants"
            >
              Previous
            </button>
            <small>Page {tenantPageNumber}</small>
            <button
              disabled={!moreTenants}
              onClick={() => setTenantPageNumber((p) => p + 1)}
              aria-label="Next Tenants"
            >
              Next
            </button>
          </div>
          <select
            id="tenant-context"
            value={tenant}
            onChange={(e) => selectTenant(e.target.value)}
          >
            {platform && <option value="">Platform overview</option>}
            {!platform && !tenants.length && (
              <option value="">No Tenant access</option>
            )}
            {tenant && !tenants.some((t) => t.id === tenant) && (
              <option value={tenant}>{tenant}</option>
            )}
            {tenants.map((t) => (
              <option key={t.id} value={t.id}>
                {t.display_name}
              </option>
            ))}
          </select>
        </div>
        <nav aria-label="Console screens">
          {[
            { label: "MAIN", ids: ["overview", "tenants"] },
            {
              label: "WORKSPACE",
              ids: ["spaces", "apps", "microapps", "databases"],
            },
            {
              label: "ADMINISTRATION",
              ids: [
                "memberships",
                "access",
                "targets",
                "plans",
                "users",
                "audit",
              ],
            },
          ].map((group) => {
            const entries = visiblePages.filter((p) =>
              group.ids.includes(p.id),
            );
            return entries.length ? (
              <div className="nav-group" key={group.label}>
                <div className="nav-label">{group.label}</div>
                {entries.map((p) => {
                  const Icon = p.icon;
                  const disabled =
                    !!p.permission &&
                    !["audit", "access"].includes(p.id) &&
                    !tenant;
                  return (
                    <button
                      key={p.id}
                      className={
                        (page === p.id ? "selected " : "") +
                        (disabled ? "nav-disabled" : "")
                      }
                      aria-current={page === p.id ? "page" : undefined}
                      title={p.label}
                      onClick={() => {
                        if (disabled && tenants.length)
                          selectTenant(tenants[0].id);
                        go(p.id);
                      }}
                    >
                      <Icon size={19} />
                      <span>{p.label}</span>
                      {page === p.id && (
                        <ChevronRight size={14} className="nav-chevron" />
                      )}
                    </button>
                  );
                })}
              </div>
            ) : null;
          })}
        </nav>
        <div className="sidebar-bottom">
          <span className="env">
            <i />
            Local development
          </span>
          <div className="sidebar-version">
            Lastapp control plane <span>v0.1</span>
          </div>
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
                  : sidebarCollapsed
                    ? "Expand sidebar"
                    : "Collapse sidebar"
              }
              aria-controls="main-navigation"
              aria-expanded={mobile ? navigationOpen : !sidebarCollapsed}
              onClick={() =>
                mobile
                  ? setNavigationOpen(true)
                  : setSidebarCollapsed(!sidebarCollapsed)
              }
            >
              <Menu size={21} />
            </button>
            <button
              className="quick-search"
              aria-label="Quick navigation"
              onClick={() => setNavigationSearch(true)}
            >
              <Search size={17} />
              <span>Quick navigation…</span>
              <kbd>⌕</kbd>
            </button>
            <span className="topbar-workspace">
              {tenantPage
                ? currentTenant?.display_name || "Choose a Tenant"
                : "Platform workspace"}
              <ChevronDown size={13} />
            </span>
          </div>
          <div className="topbar-actions">
            <button
              className="icon mode-toggle"
              aria-label={
                isDark ? "Switch to light mode" : "Switch to dark mode"
              }
              title="Toggle color mode"
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
              title="Customize appearance"
              onClick={() => setAppearanceOpen(true)}
            >
              <Palette size={20} />
              <span className="settings-dot" />
            </button>
            <div className="topbar-profile">
              <span className="avatar person">
                {me?.display_name
                  ?.split(" ")
                  .map((part: string) => part[0])
                  .slice(0, 2)
                  .join("")}
              </span>
              <div>
                <strong>{me?.display_name}</strong>
                <small>
                  {platform ? "Platform superadmin" : "Tenant member"}
                </small>
              </div>
            </div>
            <button
              className="icon"
              aria-label="Sign out"
              title="Sign out"
              onClick={signOut}
            >
              <LogOut size={17} />
            </button>
          </div>
        </header>
        <section className="content" id="main-content" tabIndex={-1}>
          <div className="page-title-row">
            <h1>{pageInfo.label}</h1>
            <div className="breadcrumbs">
              <span>Lastapp</span>
              <ChevronRight size={12} />
              <span>
                {tenantPage
                  ? currentTenant?.display_name || "Workspace"
                  : "Platform"}
              </span>
              <ChevronRight size={12} />
              <span>{pageInfo.label}</span>
            </div>
          </div>
          <div className="heading">
            <div>
              <p>{subtitle}</p>
            </div>
            <div className="heading-actions">
              <button
                className="secondary refresh"
                aria-label="Refresh"
                onClick={() => {
                  setReload((v) => v + 1);
                  loadGlobals().catch((e) => setError(e.message));
                }}
              >
                <RefreshCw size={16} />
              </button>
              {creatable &&
                [
                  "tenants",
                  "spaces",
                  "apps",
                  "microapps",
                  "databases",
                  "targets",
                  "plans",
                  "users",
                  "memberships",
                ].includes(page) &&
                (!tenantPage || tenant) && (
                  <button className="primary" onClick={create}>
                    <Plus size={17} />
                    {
                      (
                        {
                          tenants: "Create Tenant",
                          spaces: "Create Space",
                          apps: "Create App",
                          microapps: "Register micro-app",
                          databases: "Add database",
                          targets: "Add target",
                          plans: "Create plan",
                          users: "Create account",
                          memberships: "Add member",
                          access:
                            tab === "groups"
                              ? "Create group"
                              : tab === "roles"
                                ? "Create role"
                                : "Grant access",
                        } as Item
                      )[page]
                    }
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
          {tenantPage && !tenant ? (
            <div className="empty">
              <Building2 size={35} />
              <h3>Select a Tenant to get started</h3>
              <p>
                Choose a workspace from the sidebar to manage its resources.
              </p>
              {platform && (
                <button className="primary" onClick={() => go("tenants")}>
                  Manage Tenants <ArrowUpRight size={16} />
                </button>
              )}
            </div>
          ) : page === "tenant-detail" ? (
            <section className="panel workspace-section">
              <h3>{overview.tenant?.display_name}</h3>
              <p>
                Tenant operations and inventory. Tenant administrators manage
                their Spaces and Apps.
              </p>
              <dl className="workspace-facts">
                <dt>Plan</dt>
                <dd>{overview.tenant?.plan_name}</dd>
                <dt>Status</dt>
                <dd>{overview.tenant?.state}</dd>
                <dt>Spaces</dt>
                <dd>{overview.spaces}</dd>
                <dt>Apps</dt>
                <dd>{overview.apps}</dd>
                <dt>Databases</dt>
                <dd>{overview.databases}</dd>
              </dl>
            </section>
          ) : page === "overview" ? (
            <OverviewDashboard
              overview={overview}
              name={me?.display_name || "there"}
              refresh={() => setReload((v) => v + 1)}
              navigate={(dest) => {
                go("tenants");
              }}
            />
          ) : page === "access" ? (
            <AccessControl
              key={tenant}
              tenant={tenant}
              platform={platform}
              caps={caps}
              refresh={() => setReload((v) => v + 1)}
              scopeKind={tenant ? "tenant" : undefined}
            />
          ) : (
            <section className="panel list-panel">
              <div className="toolbar">
                <div className="search">
                  <Search size={16} />
                  <input
                    aria-label="Search records"
                    value={search}
                    onChange={(e) => {
                      setRecordPage(1);
                      setSearch(e.target.value);
                    }}
                    placeholder={
                      "Search " +
                      (page === "access" ? tab : pageInfo.label.toLowerCase()) +
                      "…"
                    }
                  />
                </div>
                <span>
                  {filtered.length}{" "}
                  {filtered.length === 1 ? "record" : "records"}
                </span>
              </div>
              {loading ? (
                <div className="empty">
                  <LoaderCircle className="spin" size={25} />
                  <p>Loading resources…</p>
                </div>
              ) : !filtered.length ? (
                <div className="empty">
                  <pageInfo.icon size={32} />
                  <h3>{search ? "No matching records" : "Nothing here yet"}</h3>
                  <p>
                    {search
                      ? "Try another search."
                      : "New resources will appear here once created."}
                  </p>
                </div>
              ) : (
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        {columns().map(([label]) => (
                          <th key={label}>{label}</th>
                        ))}
                        {editable && page !== "audit" && (
                          <th className="right">Actions</th>
                        )}
                      </tr>
                    </thead>
                    <tbody>
                      {filtered.map((d) => (
                        <tr key={d.id || d.user_id}>
                          {columns().map(([label, render]) => (
                            <td key={label}>{render(d)}</td>
                          ))}
                          {editable && page !== "audit" && (
                            <td className="right">
                              <div className="row-actions">{rowActions(d)}</div>
                            </td>
                          )}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              <div className="table-footer">
                <span>All changes are recorded in Activity.</span>
                <span>
                  <button
                    disabled={loading || recordPage === 1}
                    onClick={() => setRecordPage((p) => p - 1)}
                  >
                    Previous records
                  </button>{" "}
                  Page {recordPage}{" "}
                  <button
                    disabled={loading || !hasMore}
                    onClick={() => setRecordPage((p) => p + 1)}
                  >
                    Next records
                  </button>
                </span>
              </div>
            </section>
          )}
          <footer className="page-footer">
            <span>© {new Date().getFullYear()} Lastapp · Control plane</span>
            <span>Tenant → Space → App → micro-app</span>
          </footer>
        </section>
      </main>
      {appearanceOpen && (
        <AppearancePanel
          value={appearance}
          onChange={setAppearance}
          reset={resetAppearance}
          close={() => setAppearanceOpen(false)}
        />
      )}
      {navigationSearch && (
        <NavigationSearch
          pages={visiblePages}
          navigate={(id) => {
            const destination = visiblePages.find((p) => p.id === id);
            if (
              destination?.permission &&
              !["audit", "access"].includes(id) &&
              !tenant &&
              tenants.length
            )
              selectTenant(tenants[0].id);
            go(id);
          }}
          close={() => setNavigationSearch(false)}
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Check size={17} />
          {toast}
        </div>
      )}
      {modal && (
        <FormModal
          lookups={{
            plan_id: "/admin/v1/plans",
            initial_admin_user_id: "/admin/v1/users",
            user_id:
              tenant && page === "access"
                ? base + "/memberships"
                : "/admin/v1/users",
            space_id: base + "/spaces",
            app_id: base + "/apps",
            resource_id: base + "/database-resources",
            target_id: base + "/database-targets",
            group_id: base + "/teams",
            initial_manager_user_id: base + "/memberships",
          }}
          key={modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={completed}
        />
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
