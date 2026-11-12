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
} from "lucide-react";
import "./style.css";
type Item = Record<string, any>;
type Field = {
  key: string;
  label: string;
  type?: string;
  options?: Item[];
  required?: boolean;
  hint?: string;
  value?: any;
};
type Modal = {
  title: string;
  description?: string;
  fields: Field[];
  initial: Item;
  save: (v: Item) => Promise<any>;
  submit?: string;
};
class Failure extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}
async function api(
  path: string,
  method = "GET",
  body?: any,
  revision?: number,
) {
  const csrf =
    document.cookie
      .split("; ")
      .find((x) => x.startsWith("lastapp_csrf="))
      ?.split("=")[1] || "";
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf,
      ...(revision ? { "If-Match": String(revision) } : {}),
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Failure(
      res.status,
      json.error?.message || `Request failed (${res.status})`,
    );
  return json;
}
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
function FormModal({
  modal,
  close,
  done,
}: {
  modal: Modal;
  close: () => void;
  done: () => void;
}) {
  const [values, setValues] = useState<Item>(modal.initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) close();
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [busy, close]);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await modal.save(values);
      done();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }
  return (
    <div
      className="overlay"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !busy) close();
      }}
    >
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        <header>
          <div>
            <h2 id="modal-title">{modal.title}</h2>
            {modal.description && <p>{modal.description}</p>}
          </div>
          <button
            className="icon"
            aria-label="Close dialog"
            disabled={busy}
            onClick={close}
          >
            <X size={19} />
          </button>
        </header>
        <form onSubmit={submit}>
          <div className="fields">
            {modal.fields.map((f, i) => (
              <label
                className={
                  f.type === "permissions" || f.type === "note" ? "wide" : ""
                }
                key={f.key}
              >
                {f.type !== "checkbox" && (
                  <span>
                    {f.label}
                    {f.required && <b aria-hidden="true"> *</b>}
                  </span>
                )}
                {f.type === "note" ? (
                  <p className="hint">{f.hint}</p>
                ) : f.type === "checkbox" ? (
                  <span className="check-label">
                    <input
                      type="checkbox"
                      checked={!!values[f.key]}
                      onChange={(e) =>
                        setValues({ ...values, [f.key]: e.target.checked })
                      }
                    />
                    {f.label}
                  </span>
                ) : f.type === "permissions" ? (
                  <div className="permission-list">
                    {f.options?.map((o) => (
                      <label key={o.value}>
                        <input
                          type="checkbox"
                          checked={(values[f.key] || []).includes(o.value)}
                          onChange={(e) =>
                            setValues({
                              ...values,
                              [f.key]: e.target.checked
                                ? [...(values[f.key] || []), o.value]
                                : (values[f.key] || []).filter(
                                    (x: string) => x !== o.value,
                                  ),
                            })
                          }
                        />
                        <span>{o.label}</span>
                      </label>
                    ))}
                  </div>
                ) : f.type === "select" ? (
                  <select
                    aria-label={f.label}
                    required={f.required}
                    value={values[f.key] ?? ""}
                    onChange={(e) =>
                      setValues({ ...values, [f.key]: e.target.value })
                    }
                  >
                    <option value="">Choose…</option>
                    {f.options?.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    aria-label={f.label}
                    autoFocus={i === 0}
                    required={f.required}
                    min={f.type === "number" ? 0 : undefined}
                    type={f.type || "text"}
                    autoComplete={
                      f.type === "password" ? "new-password" : "off"
                    }
                    value={values[f.key] ?? ""}
                    onChange={(e) => {
                      const v: Item = { ...values, [f.key]: e.target.value };
                      if (
                        f.key === "display_name" &&
                        (!values.slug ||
                          values.slug ===
                            String(values.display_name || "")
                              .toLowerCase()
                              .replace(/[^a-z0-9]+/g, "-")
                              .slice(0, 63)) &&
                        modal.fields.some((f) => f.key === "slug")
                      )
                        v.slug = e.target.value
                          .toLowerCase()
                          .replace(/[^a-z0-9]+/g, "-")
                          .slice(0, 63);
                      setValues(v);
                    }}
                  />
                )}
                {f.hint && f.type !== "note" && <small>{f.hint}</small>}
              </label>
            ))}
          </div>
          {error && (
            <div className="error" role="alert">
              {error}
            </div>
          )}
          <footer>
            <button
              type="button"
              className="secondary"
              disabled={busy}
              onClick={close}
            >
              Cancel
            </button>
            <button className="primary" disabled={busy}>
              {busy ? (
                <LoaderCircle className="spin" size={16} />
              ) : (
                <Check size={16} />
              )}{" "}
              {busy ? "Saving…" : modal.submit || "Save changes"}
            </button>
          </footer>
        </form>
      </section>
    </div>
  );
}
function App() {
  const [me, setMe] = useState<Item | null>(null),
    [auth, setAuth] = useState<"loading" | "in" | "out">("loading");
  const [tenants, setTenants] = useState<Item[]>([]),
    [plans, setPlans] = useState<Item[]>([]),
    [users, setUsers] = useState<Item[]>([]),
    [permissions, setPermissions] = useState<Item[]>([]);
  const [tenant, setTenant] = useState(
    localStorage.getItem("lastapp-tenant") || "",
  );
  const [page, setPage] = useState(
    localStorage.getItem("lastapp-page") || "overview",
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
  const [modal, setModal] = useState<Modal | null>(null),
    [reload, setReload] = useState(0);
  const collectionKey = page + ":" + tenant + ":" + tab;
  const data = collection.key === collectionKey ? collection.items : [];
  function setData(items: Item[]) {
    setCollection({ key: collectionKey, items });
  }
  const base = `/v1/tenants/${tenant}`;
  const currentTenant = tenants.find((t) => t.id === tenant);
  const platform = !!me?.platform_admin;
  const tenantPage =
    ["spaces", "apps", "microapps", "databases", "memberships"].includes(
      page,
    ) ||
    (["access", "audit"].includes(page) && !!tenant);
  const can = (permission: string) => platform || !!caps[permission]?.available;
  const all = (permission: string) => platform || !!caps[permission]?.all;
  function go(id: string) {
    setData([]);
    setPage(id);
    localStorage.setItem("lastapp-page", id);
    setSearch("");
    setError("");
  }
  function selectTenant(id: string) {
    setData([]);
    setTenant(id);
    localStorage.setItem("lastapp-tenant", id);
    if (
      id &&
      ["overview", "tenants", "targets", "plans", "users"].includes(page)
    )
      go("spaces");
    if (!id && tenantPage) go(platform ? "overview" : "spaces");
  }
  async function loadGlobals() {
    const who = await api("/v1/me");
    setMe(who);
    setAuth("in");
    const [ts, ps, us, perms] = await Promise.all([
      who.platform_admin
        ? api("/admin/v1/tenants")
        : Promise.resolve({ items: who.tenants }),
      who.platform_admin
        ? api("/admin/v1/plans")
        : Promise.resolve({ items: [] }),
      who.platform_admin
        ? api("/admin/v1/users")
        : Promise.resolve({ items: [] }),
      api("/v1/permissions"),
    ]);
    setTenants(ts.items);
    setPlans(ps.items);
    setUsers(us.items);
    setPermissions(perms.items);
    if (!who.platform_admin && auth === "loading") go("spaces");
    if (
      !who.platform_admin &&
      (!tenant || !ts.items.some((t: Item) => t.id === tenant)) &&
      ts.items.length
    ) {
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
    if (auth !== "in") return;
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
          if (!tenant) {
            path = "/admin/v1/access-grants";
            break;
          }
          path = base + "/" + (tab === "grants" ? "access-grants" : tab);
          if (all("authorization.read")) {
            const [roles, groups] = await Promise.all([
              api(base + "/roles"),
              api(base + "/groups"),
            ]);
            aux.roles = roles.items;
            aux.groups = groups.items;
          }
          if (all("membership.read"))
            aux.members = (await api(base + "/memberships")).items;
          if (can("space.read"))
            aux.spaces = (await api(base + "/spaces")).items;
          if (can("app.read")) aux.apps = (await api(base + "/apps")).items;
          break;
        case "audit":
          path = tenant ? base + "/audit-events" : "/admin/v1/audit-events";
          break;
      }
      if (path) {
        const v = await api(path);
        if (active) setData(v.items);
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
  }, [auth, page, tenant, tab, reload]);
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
      save: (v) => api(resource, "PATCH", v, entity.revision),
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
              initial_admin_user_id:
                users.find((u) => u.id === me?.id)?.id || users[0]?.id,
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
                key: "initial_admin_user_id",
                label: "Initial Tenant administrator",
                type: "select",
                options: choose(users.filter((u) => u.state === "active")),
                required: true,
              },
            ],
            save: (v) => api("/admin/v1/tenants", "POST", v),
          });
          break;
        case "spaces": {
          const [targets, dbs] = await Promise.all([
            api(base + "/database-targets"),
            api(base + "/database-resources"),
          ]);
          setModal({
            title: "Create a Space",
            description:
              "Create a managed database or bind an existing resource.",
            initial: {
              ...init,
              mode: "managed",
              layout: "shared",
              target_id: targets.items.find((t: Item) => t.managed_enabled)?.id,
            },
            fields: [
              ...nameFields,
              {
                key: "resource_id",
                label: "Existing database (optional)",
                type: "select",
                options: dbs.items
                  .filter((d: Item) => d.state !== "retired")
                  .map((d: Item) => ({
                    value: d.id,
                    label: `${d.database_name} · ${d.layout}`,
                  })),
                hint: "If selected, the fields below are ignored.",
              },
              {
                key: "layout",
                label: "New database layout",
                type: "select",
                options: [
                  { value: "shared", label: "Shared within this Tenant" },
                  { value: "dedicated", label: "Dedicated to this Space" },
                ],
                required: true,
              },
              {
                key: "target_id",
                label: "Managed database target",
                type: "select",
                options: choose(
                  targets.items.filter((t: Item) => t.managed_enabled),
                ),
                hint: "Shared Spaces reuse the Tenant’s default database.",
              },
            ],
            save: (v) =>
              api(base + "/spaces", "POST", {
                id: v.id,
                slug: v.slug,
                display_name: v.display_name,
                database: {
                  resource_id: v.resource_id || "",
                  mode: "managed",
                  layout: v.layout,
                  target_id: v.target_id || "",
                },
              }),
          });
          break;
        }
        case "apps":
          setModal({
            title: "Create an App",
            description: "An API deployment unit within a Space.",
            initial: {
              ...init,
              space_id: support.spaces?.[0]?.id,
              compute_mode: "shared",
            },
            fields: [
              ...nameFields,
              {
                key: "space_id",
                label: "Space",
                type: "select",
                options: choose(
                  (support.spaces || []).filter(
                    (s: Item) => s.state === "active",
                  ),
                ),
                required: true,
              },
              {
                key: "compute_mode",
                label: "Compute preference",
                type: "select",
                options: [
                  { value: "shared", label: "Shared" },
                  { value: "dedicated", label: "Dedicated" },
                ],
                required: true,
              },
            ],
            save: (v) =>
              api(base + `/spaces/${v.space_id}/apps`, "POST", {
                id: v.id,
                slug: v.slug,
                display_name: v.display_name,
                compute_mode: v.compute_mode,
              }),
          });
          break;
        case "microapps":
          setModal({
            title: "Register a micro-app",
            description: "Record a client micro-app installation under an App.",
            initial: {
              ...init,
              app_id: support.apps?.[0]?.id,
              navigation_order: 0,
            },
            fields: [
              ...nameFields,
              {
                key: "app_id",
                label: "App",
                type: "select",
                options: choose(
                  (support.apps || []).filter(
                    (a: Item) => a.state === "active",
                  ),
                ),
                required: true,
              },
              {
                key: "artifact_ref",
                label: "Artifact reference",
                hint: "A library reference or bundle URL; metadata only in this phase.",
              },
              {
                key: "navigation_order",
                label: "Navigation order",
                type: "number",
              },
            ],
            save: (v) =>
              api(base + `/apps/${v.app_id}/microapps`, "POST", {
                id: v.id,
                slug: v.slug,
                display_name: v.display_name,
                artifact_ref: v.artifact_ref || "",
                navigation_order: Number(v.navigation_order || 0),
              }),
          });
          break;
        case "databases": {
          const ts = platform
            ? (await api("/admin/v1/database-targets")).items
            : (await api(base + "/database-targets")).items;
          setModal({
            title: "Add a database",
            description:
              "Provision a new database or validate an existing one without changing its data.",
            initial: {
              ...init,
              management_mode: "managed",
              layout: "dedicated",
              target_id: ts[0]?.id,
            },
            fields: [
              {
                key: "management_mode",
                label: "Method",
                type: "select",
                required: true,
                options: [
                  { value: "managed", label: "Create managed database" },
                  { value: "registered", label: "Register existing database" },
                ],
              },
              {
                key: "target_id",
                label: "Database target",
                type: "select",
                options: choose(ts),
                required: true,
              },
              {
                key: "layout",
                label: "Layout",
                type: "select",
                options: [
                  { value: "dedicated", label: "Dedicated to one Space" },
                  { value: "shared", label: "Shared within Tenant" },
                ],
                required: true,
              },
              {
                key: "database_name",
                label: "Existing database name",
                hint: "For registration only. Managed names are generated.",
              },
              { key: "username", label: "Existing database username" },
              {
                key: "password",
                label: "Existing database password",
                type: "password",
              },
            ],
            save: (v) =>
              api(base + "/database-resources", "POST", {
                id: v.id,
                management_mode: v.management_mode,
                target_id: v.target_id,
                layout: v.layout,
                ...(v.management_mode === "registered"
                  ? {
                      database_name: v.database_name,
                      credentials: {
                        username: v.username,
                        password: v.password,
                      },
                    }
                  : {}),
              }),
          });
          break;
        }
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
        case "memberships":
          setModal({
            title: "Add a Tenant member",
            description:
              "Users sign in once before they can be added. Access is assigned separately.",
            initial: {},
            fields: [
              {
                key: "user_id",
                label: "User",
                ...(platform
                  ? {
                      type: "select",
                      options: choose(
                        users.filter((u) => u.state === "active"),
                      ),
                    }
                  : { hint: "Existing user UUID" }),
                required: true,
              },
            ],
            save: (v) => api(base + "/memberships", "POST", v),
          });
          break;
        case "access":
          if (!tenant) {
            setModal({
              title: "Grant platform administration",
              description:
                "This grants full control over the platform management APIs.",
              initial: { ...init },
              fields: [
                {
                  key: "user_id",
                  label: "User",
                  type: "select",
                  options: choose(users.filter((u) => u.state === "active")),
                  required: true,
                },
              ],
              save: (v) =>
                api("/admin/v1/access-grants", "POST", {
                  id: v.id,
                  user_id: v.user_id,
                  role_id: "00000000-0000-4000-8000-000000000001",
                }),
            });
            break;
          }
          if (tab === "groups") {
            setModal({
              title: "Create a group",
              initial: init,
              fields: nameFields,
              save: (v) => api(base + "/groups", "POST", v),
            });
          } else if (tab === "roles") {
            roleModal();
          } else {
            setModal({
              title: "Grant access",
              description:
                "Permissions are additive. Leave scope empty for the whole Tenant.",
              initial: {
                ...init,
                principal_kind: "user",
                access_kind: "role",
                scope: "tenant",
              },
              fields: [
                {
                  key: "principal_kind",
                  label: "Principal type",
                  type: "select",
                  options: [
                    { value: "user", label: "User" },
                    { value: "group", label: "Group" },
                  ],
                  required: true,
                },
                {
                  key: "user_id",
                  label: "User",
                  type: "select",
                  options: choose(
                    support.members || [],
                    "display_name",
                    "user_id",
                  ),
                },
                {
                  key: "group_id",
                  label: "Group",
                  type: "select",
                  options: choose(support.groups || []),
                },
                {
                  key: "access_kind",
                  label: "Access type",
                  type: "select",
                  options: [
                    { value: "role", label: "Role bundle" },
                    { value: "permission", label: "Direct permission" },
                  ],
                  required: true,
                },
                {
                  key: "role_id",
                  label: "Role",
                  type: "select",
                  options: choose(support.roles || []),
                },
                {
                  key: "permission_code",
                  label: "Permission",
                  type: "select",
                  options: permissions
                    .filter((x) => x.kind === "tenant")
                    .map((x) => ({ value: x.code, label: x.code })),
                },
                {
                  key: "space_id",
                  label: "Space scope (optional)",
                  type: "select",
                  options: choose(support.spaces || []),
                },
                {
                  key: "app_id",
                  label: "App scope (optional)",
                  type: "select",
                  options: choose(support.apps || []),
                },
              ],
              save: (v) =>
                api(base + "/access-grants", "POST", {
                  id: v.id,
                  ...(v.principal_kind === "group"
                    ? { group_id: v.group_id }
                    : { user_id: v.user_id }),
                  ...(v.access_kind === "permission"
                    ? { permission_code: v.permission_code }
                    : { role_id: v.role_id }),
                  space_id: v.space_id || "",
                  app_id: v.app_id || "",
                }),
            });
          }
          break;
        case "plans":
          planModal();
          break;
      }
    } catch (e) {
      setError((e as Error).message);
    }
  }
  function roleModal(role?: Item) {
    setModal({
      title: role ? "Edit role" : "Create a role",
      initial: role
        ? {
            display_name: role.display_name,
            state: role.state,
            permissions: role.permissions,
          }
        : { ...initialId(), permissions: [] },
      fields: [
        { key: "display_name", label: "Name", required: true },
        ...(role
          ? [
              {
                key: "state",
                label: "State",
                type: "select",
                options: membershipStates,
                required: true,
              },
            ]
          : [{ key: "code", label: "Code", required: true }]),
        {
          key: "permissions",
          label: "Permissions",
          type: "permissions",
          options: permissions
            .filter((x) => x.kind === "tenant")
            .map((x) => ({ value: x.code, label: x.code })),
        },
      ],
      save: (v) =>
        api(
          base + "/roles" + (role ? "/" + role.id : ""),
          role ? "PATCH" : "POST",
          v,
          role?.revision,
        ),
    });
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
  function groupMember(g: Item) {
    setModal({
      title: `Add member to ${g.display_name}`,
      initial: {},
      fields: [
        {
          key: "user_id",
          label: "Tenant member",
          type: "select",
          options: choose(support.members || [], "display_name", "user_id"),
          required: true,
        },
      ],
      save: (v) => api(base + `/groups/${g.id}/members`, "POST", v, g.revision),
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
  const editable = platform || (managedPermission && can(managedPermission));
  const canOn = (permission: string, space: string, app: string) =>
    platform ||
    all(permission) ||
    (caps[permission]?.spaces || []).includes(space) ||
    (caps[permission]?.apps || []).includes(app);
  const creatable =
    platform ||
    (managedPermission &&
      (all(managedPermission) ||
        (page === "apps" &&
          (caps[managedPermission]?.spaces || []).length > 0) ||
        (page === "microapps" && can(managedPermission))));
  const filtered = data.filter((d) =>
    Object.values(d).some(
      (v) =>
        typeof v === "string" && v.toLowerCase().includes(search.toLowerCase()),
    ),
  );
  const visiblePages = pages.filter((p) =>
    p.platform
      ? platform
      : !p.permission || platform || (tenant && can(p.permission)),
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
      case "spaces":
      case "apps":
      case "microapps":
        return (
          <button
            className="text-button"
            onClick={() => {
              const collection = page;
              const fields: Field[] = [
                { key: "display_name", label: "Name", required: true },
                {
                  key: "state",
                  label: "State",
                  type: "select",
                  options: states,
                  required: true,
                },
              ];
              if (page === "apps")
                fields.push({
                  key: "compute_mode",
                  label: "Compute preference",
                  type: "select",
                  options: [
                    { value: "shared", label: "Shared" },
                    { value: "dedicated", label: "Dedicated" },
                  ],
                });
              if (page === "microapps")
                fields.push(
                  { key: "artifact_ref", label: "Artifact reference" },
                  {
                    key: "navigation_order",
                    label: "Navigation order",
                    type: "number",
                  },
                );
              setModal({
                title: "Edit " + d.display_name,
                initial: {
                  display_name: d.display_name,
                  state: d.state,
                  ...(page === "apps" ? { compute_mode: d.compute_mode } : {}),
                  ...(page === "microapps"
                    ? {
                        artifact_ref: d.artifact_ref || "",
                        navigation_order: d.navigation_order,
                      }
                    : {}),
                },
                fields,
                save: (v) =>
                  api(
                    base + "/" + collection + "/" + d.id,
                    "PATCH",
                    {
                      ...v,
                      ...(collection === "microapps"
                        ? { navigation_order: Number(v.navigation_order) }
                        : {}),
                    },
                    d.revision,
                  ),
              });
            }}
          >
            Manage <ChevronRight size={14} />
          </button>
        );
      case "databases":
        return (
          <>
            {d.management_mode === "registered" &&
              ["failed", "ready"].includes(d.state) && (
                <button
                  className="text-button"
                  onClick={() =>
                    setModal({
                      title: "Update database credentials",
                      description:
                        "Credentials are kept on the server. Saving queues a fresh verification.",
                      initial: {},
                      fields: [
                        { key: "username", label: "Username", required: true },
                        {
                          key: "password",
                          label: "Password",
                          type: "password",
                          required: true,
                        },
                      ],
                      save: (v) =>
                        api(
                          base + "/database-resources/" + d.id,
                          "PATCH",
                          { credentials: v },
                          d.revision,
                        ),
                    })
                  }
                >
                  Credentials
                </button>
              )}
            {["failed", "ready"].includes(d.state) && (
              <button
                className="text-button"
                onClick={() =>
                  action(() =>
                    api(
                      base + `/database-resources/${d.id}/retry`,
                      "POST",
                      {},
                      d.revision,
                    ),
                  )
                }
              >
                <RefreshCw size={13} />
                {d.state === "failed" ? "Retry" : "Verify"}
              </button>
            )}
            {["failed", "ready"].includes(d.state) && (
              <button
                className="text-button muted"
                onClick={() =>
                  setModal({
                    title: "Retire database resource?",
                    description:
                      "This only retires an unbound registry entry. Physical data is preserved. Bound and default resources cannot be retired.",
                    initial: {},
                    fields: [],
                    submit: "Retire resource",
                    save: () =>
                      api(
                        base + "/database-resources/" + d.id,
                        "PATCH",
                        { state: "retired" },
                        d.revision,
                      ),
                  })
                }
              >
                Retire
              </button>
            )}
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
      case "memberships":
        return (
          <button
            className="text-button"
            onClick={() =>
              action(() =>
                api(base + "/memberships/" + d.user_id, "PATCH", {
                  state: d.state === "active" ? "disabled" : "active",
                }),
              )
            }
          >
            {d.state === "active" ? "Disable" : "Enable"}
          </button>
        );
      case "access":
        if (tenant && tab === "roles")
          return d.protected ? (
            <span className="muted">Built-in</span>
          ) : (
            <button className="text-button" onClick={() => roleModal(d)}>
              Edit role
            </button>
          );
        if (tenant && tab === "groups")
          return (
            <>
              <button className="text-button" onClick={() => groupMember(d)}>
                Add member
              </button>
              <button
                className="text-button"
                onClick={() =>
                  edit(d, base + "/groups/" + d.id, "Edit group", [
                    { key: "display_name", label: "Name", required: true },
                    {
                      key: "state",
                      label: "State",
                      type: "select",
                      options: membershipStates,
                      required: true,
                    },
                  ])
                }
              >
                Edit
              </button>
            </>
          );
        return (
          <button
            className="text-button danger"
            onClick={() =>
              setModal({
                title: "Revoke access?",
                description: `Remove ${d.access_name} from ${d.principal_name}.`,
                initial: {},
                fields: [],
                submit: "Revoke access",
                save: () =>
                  api(
                    (tenant ? base : "/admin/v1") + "/access-grants/" + d.id,
                    "DELETE",
                    undefined,
                    d.revision,
                  ),
              })
            }
          >
            Revoke
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
      case "spaces":
        return [
          [
            "Space",
            (d) => (
              <>
                <strong>{d.display_name}</strong>
                <small>{d.slug}</small>
              </>
            ),
          ],
          [
            "Database",
            (d) => (
              <>
                <code>{short(d.database_resource_id)}</code>
                <small>{d.database_layout}</small>
              </>
            ),
          ],
          ["Database status", (d) => stateBadge(d.database_state)],
          ["Apps", (d) => d.app_count],
          ["Status", (d) => stateBadge(d.state)],
        ];
      case "apps":
        return [
          [
            "App",
            (d) => (
              <>
                <strong>{d.display_name}</strong>
                <small>{d.slug}</small>
              </>
            ),
          ],
          ["Space", (d) => d.space_name],
          ["Compute", (d) => <span className="tag">{d.compute_mode}</span>],
          ["Micro-apps", (d) => d.microapp_count],
          ["Status", (d) => stateBadge(d.state)],
        ];
      case "microapps":
        return [
          [
            "Micro-app",
            (d) => (
              <>
                <strong>{d.display_name}</strong>
                <small>{d.slug}</small>
              </>
            ),
          ],
          ["App", (d) => d.app_name],
          [
            "Artifact",
            (d) => (
              <code title={d.artifact_ref}>
                {d.artifact_ref || "Not linked"}
              </code>
            ),
          ],
          ["Order", (d) => d.navigation_order],
          ["Status", (d) => stateBadge(d.state)],
        ];
      case "databases":
        return [
          [
            "Database",
            (d) => (
              <>
                <code title={d.database_name}>{d.database_name}</code>
                <small>{short(d.id)}</small>
              </>
            ),
          ],
          ["Method", (d) => <span className="tag">{d.management_mode}</span>],
          ["Layout", (d) => d.layout],
          [
            "Status",
            (d) => (
              <>
                {stateBadge(d.state)}
                {d.last_error_message && (
                  <small className="failure-note" title={d.last_error_message}>
                    {d.last_error_message}
                  </small>
                )}
              </>
            ),
          ],
          ["Attempts", (d) => d.attempts],
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
      case "access":
        if (tenant && tab === "roles")
          return [
            [
              "Role",
              (d) => (
                <>
                  <strong>{d.display_name}</strong>
                  <small>{d.code}</small>
                </>
              ),
            ],
            [
              "Permissions",
              (d) => (
                <span className="wrap-tags">
                  {d.permissions.map((p: string) => (
                    <code key={p}>{p}</code>
                  ))}
                </span>
              ),
            ],
            ["Status", (d) => stateBadge(d.state)],
          ];
        if (tenant && tab === "groups")
          return [
            [
              "Group",
              (d) => (
                <>
                  <strong>{d.display_name}</strong>
                  <small>{d.slug}</small>
                </>
              ),
            ],
            [
              "Members",
              (d) => (
                <div className="wrap-tags">
                  {d.members.length
                    ? d.members.map((m: Item) => (
                        <span className="tag" key={m.user_id}>
                          {m.display_name}
                          {editable && (
                            <button
                              className="tiny"
                              aria-label={"Remove " + m.display_name}
                              onClick={() =>
                                action(() =>
                                  api(
                                    base +
                                      `/groups/${d.id}/members/${m.user_id}`,
                                    "DELETE",
                                    undefined,
                                    d.revision,
                                  ),
                                )
                              }
                            >
                              <X size={11} />
                            </button>
                          )}
                        </span>
                      ))
                    : "No members"}
                </div>
              ),
            ],
            ["Status", (d) => stateBadge(d.state)],
          ];
        return [
          [
            "Principal",
            (d) => (
              <>
                <strong>{d.principal_name}</strong>
                <small>{d.group_id ? "Group" : "User"}</small>
              </>
            ),
          ],
          ["Access", (d) => d.access_name],
          ["Scope", (d) => (tenant ? d.scope_name : "Entire platform")],
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
  if (auth === "out")
    return (
      <div className="welcome">
        <div className="welcome-card">
          <span className="brand-mark">
            <Layers3 size={30} />
          </span>
          <span className="eyebrow">LASTAPP CONTROL PLANE</span>
          <h1>
            Your platform.
            <br />
            One place to manage it.
          </h1>
          <p>
            Provision Tenants, connect databases and organize every Space and
            App.
          </p>
          {error && <div className="error">{error}</div>}
          <a className="primary" href="/auth/login">
            Sign in to Lastapp <ArrowUpRight size={18} />
          </a>
          <small>Local development environment</small>
        </div>
        <div className="welcome-art">
          <div className="orbit o1" />
          <div className="orbit o2" />
          <div className="orbit o3" />
          <Layers3 size={100} />
          <span className="art-label">A foundation for every app.</span>
        </div>
      </div>
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
      access: "Shared roles, groups and scoped permission grants.",
      audit: "A record of administrative changes and provisioning events.",
    } as Item
  )[page];
  return (
    <div className="shell">
      <aside>
        <a
          className="brand"
          onClick={() => go(platform ? "overview" : "spaces")}
        >
          <span className="brand-mark">
            <Layers3 size={23} />
          </span>
          <b>
            lastapp<span>CONTROL PLANE</span>
          </b>
        </a>
        <div className="context-picker">
          <label htmlFor="tenant-context">WORKSPACE</label>
          <select
            id="tenant-context"
            value={tenant}
            onChange={(e) => selectTenant(e.target.value)}
          >
            {platform && <option value="">Platform overview</option>}
            {!platform && !tenants.length && (
              <option value="">No Tenant access</option>
            )}
            {tenants.map((t) => (
              <option key={t.id} value={t.id}>
                {t.display_name}
              </option>
            ))}
          </select>
        </div>
        <nav>
          {visiblePages.map((p) => {
            const Icon = p.icon;
            const disabled =
              !!p.permission && !["audit", "access"].includes(p.id) && !tenant;
            return (
              <button
                key={p.id}
                className={
                  (page === p.id ? "selected " : "") +
                  (disabled ? "nav-disabled" : "")
                }
                onClick={() => {
                  if (disabled && tenants.length) {
                    selectTenant(tenants[0].id);
                  }
                  go(p.id);
                }}
              >
                <Icon size={18} />
                <span>{p.label}</span>
                {page === p.id && <i />}
              </button>
            );
          })}
        </nav>
        <div className="sidebar-bottom">
          <span className="env">
            <i />
            Local development
          </span>
          <div className="profile">
            <span className="avatar person">{me?.display_name[0]}</span>
            <div>
              <strong>{me?.display_name}</strong>
              <small>
                {platform ? "Platform superadmin" : "Tenant member"}
              </small>
            </div>
            <button
              className="icon"
              aria-label="Sign out"
              onClick={() =>
                api("/auth/logout", "POST", {}).then(() => {
                  setAuth("out");
                  setMe(null);
                })
              }
            >
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <div>
            <span>
              {tenantPage
                ? currentTenant?.display_name || "Choose a Tenant"
                : "Platform"}
            </span>
            <ChevronRight size={13} />
            <strong>{pageInfo.label}</strong>
          </div>
          <span className="version">
            Lastapp <span>v0.1</span>
          </span>
        </header>
        <section className="content">
          <div className="heading">
            <div>
              <div className="eyebrow">
                {tenantPage ? "TENANT WORKSPACE" : "PLATFORM MANAGEMENT"}
              </div>
              <h1>{pageInfo.label}</h1>
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
                  "memberships",
                  "access",
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
          ) : page === "overview" ? (
            <>
              <div className="stats">
                {[
                  {
                    key: "tenants",
                    label: "Tenants",
                    icon: Building2,
                    page: "tenants",
                  },
                  {
                    key: "spaces",
                    label: "Spaces",
                    icon: FolderOpen,
                    page: "spaces",
                  },
                  { key: "apps", label: "Apps", icon: AppWindow, page: "apps" },
                  {
                    key: "database_resources",
                    label: "Databases",
                    icon: Database,
                    page: "databases",
                  },
                ].map(({ key, label, icon: Icon, page: dest }) => (
                  <button
                    className="stat"
                    key={key}
                    onClick={() => {
                      if (dest !== "tenants" && !tenant && tenants.length)
                        selectTenant(tenants[0].id);
                      go(dest);
                    }}
                  >
                    <span>
                      {label}
                      <Icon size={18} />
                    </span>
                    <strong>{overview[key] ?? "—"}</strong>
                    <small>
                      View {label.toLowerCase()} <ArrowUpRight size={13} />
                    </small>
                  </button>
                ))}
              </div>
              <div className="overview-grid">
                <section className="panel quick-start">
                  <div className="panel-heading">
                    <h2>Your next building block</h2>
                    <span className="tag purple">Getting started</span>
                  </div>
                  <p>
                    Start with a Tenant. Give it a Space, connect a database,
                    then organize the Apps your teams will use.
                  </p>
                  <div className="hierarchy">
                    <div>
                      <Building2 size={19} />
                      <b>Tenant</b>
                      <small>The company</small>
                    </div>
                    <ChevronRight size={16} />
                    <div>
                      <FolderOpen size={19} />
                      <b>Space</b>
                      <small>Data & rules</small>
                    </div>
                    <ChevronRight size={16} />
                    <div>
                      <AppWindow size={19} />
                      <b>App</b>
                      <small>API & clients</small>
                    </div>
                  </div>
                  <button className="text-button" onClick={() => go("tenants")}>
                    Manage your Tenants <ArrowUpRight size={15} />
                  </button>
                </section>
                <section className="panel health">
                  <div className="panel-heading">
                    <h2>Database health</h2>
                    <Database size={18} />
                  </div>
                  {["ready", "pending", "provisioning", "failed"].map(
                    (state) => (
                      <div className="health-row" key={state}>
                        {stateBadge(state)}
                        <strong>
                          {overview.database_states?.find(
                            (s: Item) => s.state === state,
                          )?.count || 0}
                        </strong>
                      </div>
                    ),
                  )}
                </section>
              </div>
              <section className="panel">
                <div className="panel-heading">
                  <div>
                    <h2>Recent database resources</h2>
                    <p>Provisioning status across the platform</p>
                  </div>
                  <span className="tag">Live updates</span>
                </div>
                {overview.recent_databases?.length ? (
                  <div className="table-wrap">
                    <table>
                      <thead>
                        <tr>
                          <th>Database</th>
                          <th>Tenant</th>
                          <th>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {overview.recent_databases.map((d: Item) => (
                          <tr key={d.id}>
                            <td>
                              <code>{d.database_name}</code>
                            </td>
                            <td>{d.tenant_name}</td>
                            <td>{stateBadge(d.state)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className="empty compact">
                    <Database size={28} />
                    <h3>No databases yet</h3>
                    <p>Create your first Tenant and Space to provision one.</p>
                  </div>
                )}
              </section>
            </>
          ) : (
            <section className="panel list-panel">
              {page === "access" && tenant && (
                <div className="tabs">
                  {["grants", "roles", "groups"].map((t) => (
                    <button
                      className={tab === t ? "active" : ""}
                      key={t}
                      onClick={() => {
                        setData([]);
                        setTab(t);
                        setSearch("");
                      }}
                    >
                      {t === "grants"
                        ? "Access grants"
                        : t[0].toUpperCase() + t.slice(1)}
                    </button>
                  ))}
                </div>
              )}
              <div className="toolbar">
                <div className="search">
                  <Search size={16} />
                  <input
                    aria-label="Search records"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
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
                <span>
                  {page === "audit"
                    ? "Latest 150 events"
                    : "All changes are recorded in Activity."}
                </span>
                <span>Lastapp control plane</span>
              </div>
            </section>
          )}
          <footer className="page-footer">
            <span>
              <Layers3 size={13} /> Built on a shared foundation.
            </span>
            <span>Tenant → Space → App → micro-app</span>
          </footer>
        </section>
      </main>
      {toast && (
        <div className="toast" role="status">
          <Check size={17} />
          {toast}
        </div>
      )}
      {modal && (
        <FormModal
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
