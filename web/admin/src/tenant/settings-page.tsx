import { AgentAccess } from "../agent-access";
import { api } from "../api";
import { Collection } from "../collection";
import { confirmAction } from "../confirm";
import { statusColumn, type TenantScreen } from "./screen";

// Tenant settings: General, Databases and Agent access.
export function TenantSettings({ t }: { t: TenantScreen }) {
  const { act, all, base, busy, edit, me, profile, route, setModal, tenant } =
    t;
  return (
    <>
      {route.tab === "agents" && (
        <AgentAccess key={tenant} me={me} tenant={tenant} />
      )}
      {route.tab === "general" && (
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
      {route.tab === "databases" && (
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
                              base + "/database-resources/" + r.id + "/retry",
                              "POST",
                              {},
                              r.revision,
                            ),
                          )
                        }
                      >
                        {r.state === "failed" ? "Retry" : "Recheck connection"}
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
  );
}
