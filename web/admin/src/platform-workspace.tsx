import { useBuiltinRoles } from "./console-config";
import { useEffect, useState, type ReactNode } from "react";
import {
  Building2,
  Users,
  ScrollText,
  Settings,
  Plus,
  RefreshCw,
} from "lucide-react";
import { api } from "./api";
import { Collection } from "./collection";
import { ConsoleShell } from "./console-shell";
import { useConsoleURL } from "./console-url";
import { FormModal, type Modal, type Field } from "./form-modal";
import { PasswordScreen } from "./login";
import { useAppearance } from "./appearance";
import { useAction } from "./use-action";
import {
  accountFields,
  accountPayload,
  initialPasswordField,
  nameFields,
  stateOptions,
  status,
} from "./console-forms";
import type { Item, Me, Page, TenantOverview } from "./api-types";
const nav = [
  { id: "tenants", label: "Tenants", icon: Building2 },
  { id: "users", label: "Users", icon: Users },
  { id: "activity", label: "Activity", icon: ScrollText },
  { id: "settings", label: "Settings", icon: Settings },
];
export function PlatformWorkspace({
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
  const roles = useBuiltinRoles();
  const [path, navigate] = useConsoleURL(),
    parts = path.split("/").filter(Boolean),
    page = parts[2] || "tenants",
    tenant = page === "tenants" ? parts[3] || "" : "",
    tab = parts[4] || "overview",
    settingsTab = parts[3] || "servers";
  const [overview, setOverview] = useState<TenantOverview>({}),
    [reload, setReload] = useState(0),
    [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState(""),
    [plans, setPlans] = useState<Item[]>([]);
  const refresh = () => setReload((r) => r + 1);
  const { busy, run: act } = useAction(path, {
    onSuccess: refresh,
    onError: setError,
  });
  function go(page: string) {
    navigate("/console/platform/" + page);
  }
  useEffect(() => {
    if (!path.startsWith("/console/platform/"))
      navigate("/console/platform/tenants", true);
    setModal(null);
    setError("");
  }, [path]);
  useEffect(() => {
    let live = true;
    api<Page>("/admin/v1/plans")
      .then((v) => {
        if (live) setPlans(v.items);
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [reload]);
  useEffect(() => {
    setOverview({});
    if (!tenant) return;
    let live = true;
    api<TenantOverview>("/v1/tenants/" + tenant + "/overview")
      .then((v) => {
        if (live) setOverview(v);
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [tenant, reload]);
  const profile = overview.tenant || {};
  function done() {
    setModal(null);
    refresh();
  }
  function addAdmin() {
    setModal({
      title: "Add administrator",
      submit: "Add administrator",
      description:
        "Grant administrator access using a new or existing account.",
      initial: { mode: "existing" },
      fields: accountFields(),
      save: (v) =>
        api(
          `/admin/v1/tenants/${tenant}/administrators`,
          "POST",
          accountPayload(v),
        ),
    });
  }
  function createTenant() {
    setModal({
      title: "Create Tenant",
      submit: "Create Tenant",
      initial: {
        id: crypto.randomUUID(),
        plan_id: plans[0]?.id,
        admin_mode: "new",
      },
      fields: [
        ...nameFields,
        {
          key: "plan_id",
          label: "Plan",
          type: "select",
          lookup: "/admin/v1/plans",
          options: plans.map((p) => ({ value: p.id, label: p.display_name })),
          required: true,
        },
        ...accountFields("admin_"),
      ],
      save: (v) => {
        const chosen = accountPayload(v, "admin_");
        return api("/admin/v1/tenants", "POST", {
          id: v.id,
          slug: v.slug,
          display_name: v.display_name,
          plan_id: v.plan_id,
          ...(chosen.user_id
            ? { initial_admin_user_id: chosen.user_id }
            : { initial_admin: chosen.account }),
        });
      },
    });
  }
  function createUser() {
    setModal({
      title: "Create account",
      submit: "Create account",
      initial: {},
      fields: [
        { key: "display_name", label: "Name", required: true },
        { key: "email", label: "Email", type: "email", required: true },
        initialPasswordField(),
      ],
      save: (v) => api("/admin/v1/users", "POST", v),
    });
  }
  function addToTenant(user: Item) {
    setModal({
      title: "Add to Tenant",
      submit: "Add to Tenant",
      description: `Add ${user.display_name} as an ordinary Tenant member.`,
      initial: {},
      fields: [
        {
          key: "tenant_id",
          label: "Tenant",
          type: "select",
          lookup: "/admin/v1/tenants",
          required: true,
        },
      ],
      save: (v) =>
        api(`/v1/tenants/${v.tenant_id}/memberships`, "POST", {
          user_id: user.id,
        }),
    });
  }
  function editTenant() {
    setModal({
      title: "Edit Tenant",
      submit: "Save",
      initial: {
        ...profile,
        default_layout: profile.default_layout || "shared",
      },
      fields: [
        { key: "display_name", label: "Name", required: true },
        {
          key: "plan_id",
          label: "Plan",
          lookup: "/admin/v1/plans",
          type: "select",
          options: plans.map((p) => ({ value: p.id, label: p.display_name })),
          required: true,
        },
        {
          key: "state",
          label: "Status",
          type: "select",
          options: stateOptions,
          required: true,
        },
        {
          key: "default_layout",
          label: "Default database layout",
          type: "select",
          options: [
            { value: "shared", label: "Shared within Tenant" },
            { value: "dedicated", label: "Dedicated per Space" },
          ],
          advanced: true,
        },
      ],
      save: (v) =>
        api(
          "/admin/v1/tenants/" + tenant,
          "PATCH",
          {
            display_name: v.display_name,
            plan_id: v.plan_id,
            state: v.state,
            default_layout: v.default_layout,
          },
          profile.revision,
        ),
    });
  }
  function planForm(plan?: Item) {
    const flags = [
      "shared_database",
      "dedicated_database",
      "managed_database",
      "registered_database",
      "dedicated_compute",
    ];
    const initial: Item = plan
      ? { ...plan }
      : {
          id: crypto.randomUUID(),
          default_layout: "shared",
          allow_shared_database: true,
          allow_managed_database: true,
          allow_registered_database: true,
        };
    flags.forEach((f) => (initial["allow_" + f] = !!initial["allow_" + f]));
    setModal({
      title: plan ? "Edit plan" : "Create plan",
      submit: plan ? "Save" : "Create plan",
      initial,
      fields: [
        { key: "display_name", label: "Name", required: true },
        ...(!plan
          ? [{ key: "code", label: "Code", required: true, advanced: true }]
          : []),
        ...["spaces", "apps", "databases"].map((x) => ({
          key: "max_" + x,
          label: "Maximum " + x,
          type: "number",
          hint: "Blank means unlimited.",
        })),
        {
          key: "default_layout",
          label: "Default database layout",
          type: "select",
          options: [
            { value: "shared", label: "Shared" },
            { value: "dedicated", label: "Dedicated" },
          ],
          required: true,
          advanced: true,
        },
        ...flags.map((f) => ({
          key: "allow_" + f,
          label: "Allow " + f.replaceAll("_", " "),
          type: "checkbox",
          advanced: true,
        })),
      ],
      save: (v) => {
        const out: Item = {
          id: v.id,
          code: v.code,
          display_name: v.display_name,
          default_layout: v.default_layout,
        };
        flags.forEach((f) => (out["allow_" + f] = !!v["allow_" + f]));
        ["spaces", "apps", "databases"].forEach(
          (x) =>
            (out["max_" + x] =
              v["max_" + x] === "" || v["max_" + x] == null
                ? null
                : Number(v["max_" + x])),
        );
        return api(
          "/admin/v1/plans" + (plan ? "/" + plan.id : ""),
          plan ? "PATCH" : "POST",
          out,
          plan?.revision,
        );
      },
    });
  }
  function serverForm(server?: Item) {
    const fields: Field[] = [
      { key: "display_name", label: "Name", required: true },
      ...(!server
        ? [
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
          ]
        : [
            {
              key: "state",
              label: "Status",
              type: "select",
              options: ["active", "disabled"].map((value) => ({
                value,
                label: value,
              })),
              required: true,
            },
          ]),
      {
        key: "managed_enabled",
        label: "Allow managed database creation",
        type: "checkbox",
        wide: true,
      },
      {
        key: "username",
        label: "Provisioning username",
        hint: server ? "Leave blank to keep existing credentials." : undefined,
      },
      { key: "password", label: "Provisioning password", type: "password" },
    ];
    setModal({
      title: server ? "Edit database server" : "Add database server",
      submit: server ? "Save" : "Add database server",
      initial: server
        ? { ...server, managed_enabled: !!server.managed_enabled }
        : {
            id: crypto.randomUUID(),
            port: 3306,
            tls_mode: "verify_identity",
            managed_enabled: true,
          },
      fields,
      save: (v) =>
        api(
          "/admin/v1/database-targets" + (server ? "/" + server.id : ""),
          server ? "PATCH" : "POST",
          {
            display_name: v.display_name,
            managed_enabled: !!v.managed_enabled,
            ...(server
              ? { state: v.state }
              : {
                  id: v.id,
                  canonical_host: v.canonical_host,
                  port: Number(v.port),
                  tls_mode: v.tls_mode,
                }),
            ...(v.username
              ? { credentials: { username: v.username, password: v.password } }
              : {}),
          },
          server?.revision,
        ),
    });
  }
  const title = tenant
    ? profile.display_name || "Tenant"
    : page === "account"
      ? "My account"
      : nav.find((p) => p.id === page)?.label || "Tenants";
  const columnsName: [string, (r: Item) => ReactNode] = [
    "Name",
    (r) => (
      <>
        <strong>{r.display_name}</strong>
        {r.email && <small>{r.email}</small>}
      </>
    ),
  ];
  const columnsStatus: [string, (r: Item) => ReactNode] = [
    "Status",
    (r) => status(r.state),
  ];
  const addButton =
    tenant && tab === "administrators" ? (
      <button className="primary" onClick={addAdmin}>
        <Plus size={16} />
        Add administrator
      </button>
    ) : !tenant && page === "tenants" ? (
      <button className="primary" onClick={createTenant}>
        <Plus size={16} />
        Create Tenant
      </button>
    ) : page === "users" ? (
      <button className="primary" onClick={createUser}>
        <Plus size={16} />
        Create account
      </button>
    ) : page === "settings" ? (
      <button
        className="primary"
        onClick={() => (settingsTab === "plans" ? planForm() : serverForm())}
      >
        <Plus size={16} />
        {settingsTab === "plans" ? "Create plan" : "Add database server"}
      </button>
    ) : null;
  return (
    <ConsoleShell
      me={me}
      kind="Platform"
      title={title}
      nav={nav}
      active={page === "account" ? "" : page}
      navigate={go}
      appearanceControls={appearanceControls}
      account={() => go("account")}
      signOut={signOut}
      breadcrumbs={
        tenant ? (
          <>
            <button className="text-button" onClick={() => go("tenants")}>
              Tenants
            </button>
            <span> / {profile.display_name || "Tenant"}</span>
          </>
        ) : (
          "Platform"
        )
      }
      actions={
        <>
          <button className="icon" aria-label="Refresh" onClick={refresh}>
            <RefreshCw size={16} />
          </button>
          {addButton}
        </>
      }
    >
      {error && (
        <div className="error" role="alert">
          {error}
          <button
            className="tiny"
            onClick={() => setError("")}
            aria-label="Dismiss error"
          >
            ×
          </button>
        </div>
      )}
      {tenant ? (
        <>
          <div className="tabs workspace-tabs">
            {["overview", "administrators", "settings"].map((id) => (
              <button
                key={id}
                className={tab === id ? "active" : ""}
                onClick={() =>
                  navigate(`/console/platform/tenants/${tenant}/${id}`)
                }
              >
                {id[0].toUpperCase() + id.slice(1)}
              </button>
            ))}
          </div>
          {tab === "overview" && (
            <>
              <section className="panel workspace-section">
                <dl className="workspace-facts">
                  <dt>Plan</dt>
                  <dd>{profile.plan_name}</dd>
                  <dt>Status</dt>
                  <dd>{profile.state}</dd>
                  <dt>Spaces / Apps / Databases</dt>
                  <dd>
                    {overview.spaces} / {overview.apps} / {overview.databases}
                  </dd>
                </dl>
                <button
                  className="primary"
                  onClick={() =>
                    navigate(
                      `/console/platform/tenants/${tenant}/administrators`,
                    )
                  }
                >
                  Manage administrators
                </button>
              </section>
              <h2 className="section-title">Recent Apps</h2>
              <Collection
                key={tenant + "inventory"}
                path={`/v1/tenants/${tenant}/apps`}
                reload={reload}
                columns={[
                  columnsName,
                  ["Space", (r) => r.space_name],
                  columnsStatus,
                ]}
              />
            </>
          )}
          {tab === "administrators" && (
            <Collection
              key={tenant + "admins"}
              path={`/admin/v1/tenants/${tenant}/administrators`}
              reload={reload}
              columns={[
                columnsName,
                columnsStatus,
                ["Account", (r) => r.user_state],
              ]}
              actions={(r) => (
                <button
                  className="text-button danger"
                  disabled={busy}
                  onClick={() =>
                    act(() =>
                      api(
                        `/v1/tenants/${tenant}/role-assignments/${r.id}`,
                        "DELETE",
                        undefined,
                        r.revision,
                      ),
                    )
                  }
                >
                  Remove administrator
                </button>
              )}
            />
          )}
          {tab === "settings" && (
            <section className="panel workspace-section">
              <p>
                {profile.plan_name} · {profile.state}
              </p>
              <div className="section-actions">
                <button className="secondary" onClick={editTenant}>
                  Edit Tenant
                </button>
                <button
                  className="text-button"
                  onClick={() => go("activity/" + tenant)}
                >
                  View Tenant activity
                </button>
              </div>
            </section>
          )}
        </>
      ) : page === "tenants" ? (
        <Collection
          path="/admin/v1/tenants"
          reload={reload}
          columns={[
            columnsName,
            ["Plan", (r) => r.plan_name],
            ["Spaces", (r) => r.space_count],
            ["Apps", (r) => r.app_count],
            columnsStatus,
          ]}
          actions={(r) => (
            <button
              className="text-button"
              onClick={() =>
                navigate("/console/platform/tenants/" + r.id + "/overview")
              }
            >
              Open
            </button>
          )}
        />
      ) : page === "users" ? (
        <Collection
          path="/admin/v1/users"
          reload={reload}
          columns={[
            columnsName,
            [
              "Sign-in",
              (r) =>
                r.oidc_issuer === "urn:lastapp:local"
                  ? "Email/password"
                  : "Single sign-on",
            ],
            columnsStatus,
            [
              "Platform admin",
              (r) => (
                <input
                  type="checkbox"
                  aria-label={"Platform administrator for " + r.display_name}
                  checked={!!r.platform_admin}
                  disabled={
                    busy || (!r.platform_assignment_id && r.state !== "active")
                  }
                  onChange={() =>
                    act(() =>
                      r.platform_assignment_id
                        ? api(
                            "/admin/v1/role-assignments/" +
                              r.platform_assignment_id,
                            "DELETE",
                            undefined,
                            r.platform_assignment_revision,
                          )
                        : api("/admin/v1/role-assignments", "POST", {
                            id: crypto.randomUUID(),
                            user_id: r.id,
                            role_id: roles.platform,
                          }),
                    )
                  }
                />
              ),
            ],
          ]}
          actions={(r) => (
            <>
              {!r.platform_admin && r.state === "active" && (
                <button className="text-button" onClick={() => addToTenant(r)}>
                  Add to Tenant
                </button>
              )}
              <button
                className="text-button"
                disabled={busy}
                onClick={() =>
                  act(() =>
                    api(
                      "/admin/v1/users/" + r.id,
                      "PATCH",
                      { state: r.state === "active" ? "disabled" : "active" },
                      r.revision,
                    ),
                  )
                }
              >
                {r.state === "active" ? "Disable account" : "Enable account"}
              </button>
            </>
          )}
        />
      ) : page === "activity" ? (
        <Collection
          path={
            parts[3]
              ? "/v1/tenants/" + parts[3] + "/audit-events"
              : "/admin/v1/audit-events"
          }
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
      ) : page === "settings" ? (
        <>
          <div className="tabs workspace-tabs">
            {[
              ["servers", "Database servers"],
              ["plans", "Plans"],
            ].map(([id, label]) => (
              <button
                key={id}
                className={settingsTab === id ? "active" : ""}
                onClick={() => go("settings/" + id)}
              >
                {label}
              </button>
            ))}
          </div>
          {settingsTab === "plans" ? (
            <Collection
              path="/admin/v1/plans"
              reload={reload}
              columns={[
                columnsName,
                [
                  "Limits",
                  (r) =>
                    `${r.max_spaces ?? "∞"} Spaces · ${r.max_apps ?? "∞"} Apps · ${r.max_databases ?? "∞"} databases`,
                ],
              ]}
              actions={(r) => (
                <button className="text-button" onClick={() => planForm(r)}>
                  Edit plan
                </button>
              )}
            />
          ) : (
            <Collection
              path="/admin/v1/database-targets"
              reload={reload}
              columns={[
                columnsName,
                ["Endpoint", (r) => r.canonical_host + ":" + r.port],
                columnsStatus,
              ]}
              actions={(r) => (
                <button className="text-button" onClick={() => serverForm(r)}>
                  Edit server
                </button>
              )}
            />
          )}
        </>
      ) : page === "account" ? (
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
      ) : null}
      {modal && (
        <FormModal
          key={path + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={done}
          lookups={{}}
        />
      )}
    </ConsoleShell>
  );
}
import { AgentAccess } from "./agent-access";
