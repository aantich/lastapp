import { api } from "../api";
import { num, text } from "../api-types";
import { Collection } from "../ui";
import { useBuiltinRoles } from "../console-config";
import { addToTenant, editTenant, planForm, serverForm } from "./forms";
import { columnsName, columnsStatus, type PlatformScreen } from "./screen";

// One Tenant's Overview, Administrators and Settings tabs.
export function PlatformTenant({ p }: { p: PlatformScreen }) {
  const { act, busy, go, overview, profile, route, tenant } = p;
  const { tab } = route;
  return (
    <>
      <div className="tabs workspace-tabs">
        {["overview", "administrators", "settings"].map((id) => (
          <button
            key={id}
            className={tab === id ? "active" : ""}
            onClick={() => go("tenants", tenant, id)}
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
              <dd>{text(profile.plan_name)}</dd>
              <dt>Status</dt>
              <dd>{text(profile.state)}</dd>
              <dt>Spaces / Apps / Databases</dt>
              <dd>
                {overview.spaces} / {overview.apps} / {overview.databases}
              </dd>
            </dl>
            <button
              className="primary"
              onClick={() => go("tenants", tenant, "administrators")}
            >
              Manage administrators
            </button>
          </section>
          <h2 className="section-title">Recent Apps</h2>
          <Collection
            key={tenant + "inventory"}
            path={`/v1/tenants/${tenant}/apps`}
            columns={[
              columnsName,
              ["Space", (r) => text(r.space_name)],
              columnsStatus,
            ]}
          />
        </>
      )}
      {tab === "administrators" && (
        <Collection
          key={tenant + "admins"}
          path={`/admin/v1/tenants/${tenant}/administrators`}
          columns={[
            columnsName,
            columnsStatus,
            ["Account", (r) => text(r.user_state)],
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
                    num(r.revision),
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
            {text(profile.plan_name)} · {text(profile.state)}
          </p>
          <div className="section-actions">
            <button className="secondary" onClick={() => editTenant(p)}>
              Edit Tenant
            </button>
            <button
              className="text-button"
              onClick={() => go("activity", tenant)}
            >
              View Tenant activity
            </button>
          </div>
        </section>
      )}
    </>
  );
}
// Every Tenant on the platform.
export function TenantsPage({ p }: { p: PlatformScreen }) {
  const { go } = p;
  return (
    <Collection
      path="/admin/v1/tenants"
      columns={[
        columnsName,
        ["Plan", (r) => text(r.plan_name)],
        ["Spaces", (r) => text(r.space_count)],
        ["Apps", (r) => text(r.app_count)],
        columnsStatus,
      ]}
      actions={(r) => (
        <button
          className="text-button"
          onClick={() => go("tenants", text(r.id), "overview")}
        >
          Open
        </button>
      )}
    />
  );
}
// Every account, with platform administrator and sign-in controls.
export function UsersPage({ p }: { p: PlatformScreen }) {
  const { act, busy } = p;
  const roles = useBuiltinRoles();
  return (
    <Collection
      path="/admin/v1/users"
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
                        num(r.platform_assignment_revision),
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
            <button className="text-button" onClick={() => addToTenant(p, r)}>
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
                  num(r.revision),
                ),
              )
            }
          >
            {r.state === "active" ? "Disable account" : "Enable account"}
          </button>
        </>
      )}
    />
  );
}
// Database servers and plans.
export function PlatformSettings({ p }: { p: PlatformScreen }) {
  const { go, route } = p;
  const { settingsTab } = route;
  return (
    <>
      <div className="tabs workspace-tabs">
        {[
          ["servers", "Database servers"],
          ["plans", "Plans"],
        ].map(([id, label]) => (
          <button
            key={id}
            className={settingsTab === id ? "active" : ""}
            onClick={() => go("settings", id)}
          >
            {label}
          </button>
        ))}
      </div>
      {settingsTab === "plans" ? (
        <Collection
          path="/admin/v1/plans"
          columns={[
            columnsName,
            [
              "Limits",
              (r) =>
                `${r.max_spaces ?? "∞"} Spaces · ${r.max_apps ?? "∞"} Apps · ${r.max_databases ?? "∞"} databases`,
            ],
          ]}
          actions={(r) => (
            <button className="text-button" onClick={() => planForm(p, r)}>
              Edit plan
            </button>
          )}
        />
      ) : (
        <Collection
          path="/admin/v1/database-targets"
          columns={[
            columnsName,
            ["Endpoint", (r) => r.canonical_host + ":" + r.port],
            columnsStatus,
          ]}
          actions={(r) => (
            <button className="text-button" onClick={() => serverForm(p, r)}>
              Edit server
            </button>
          )}
        />
      )}
    </>
  );
}
