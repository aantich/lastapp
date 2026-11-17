import { AgentAccess } from "../agent-access";
import { api } from "../api";
import { Collection, confirmAction } from "../ui";
import { num, text } from "../api-types";
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
            <dd>{text(profile.display_name)}</dd>
            <dt>Plan</dt>
            <dd>{text(profile.plan_name)}</dd>
            <dt>Status</dt>
            <dd>{text(profile.state)}</dd>
            <dt>Spaces / Apps / Databases</dt>
            <dd>
              {text(profile.max_spaces ?? "Unlimited")} /{" "}
              {text(profile.max_apps ?? "Unlimited")} /{" "}
              {text(profile.max_databases ?? "Unlimited")}
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
                  <strong>{text(r.database_name)}</strong>
                  <small>
                    {text(r.management_mode)} · {text(r.layout)}
                  </small>
                </>
              ),
            ],
            statusColumn,
            ["Last result", (r) => text(r.last_error_message) || "—"],
          ]}
          actions={
            all("database.manage")
              ? (r) => (
                  <>
                    {["ready", "failed"].includes(text(r.state)) && (
                      <button
                        className="text-button"
                        disabled={busy}
                        onClick={() =>
                          act(() =>
                            api(
                              base + "/database-resources/" + r.id + "/retry",
                              "POST",
                              {},
                              num(r.revision),
                            ),
                          )
                        }
                      >
                        {r.state === "failed" ? "Retry" : "Recheck connection"}
                      </button>
                    )}
                    {r.management_mode === "registered" &&
                      !["provisioning", "retired"].includes(text(r.state)) && (
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
                                  num(r.revision),
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
                                  num(r.revision),
                                ),
                            })
                          }
                        >
                          Set schema credentials
                        </button>
                      )}
                    {!["provisioning", "retired"].includes(text(r.state)) && (
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
                                num(r.revision),
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
