import { AgentAccess } from "../agent-access";
import { api } from "../api";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Collection,
  FactList,
  confirmAction,
} from "../ui";
import { num, text } from "../api-types";
import { status } from "../console-forms";
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
        <Card>
          <CardHeader
            title="General"
            description="The company name is yours to change; plan and status are set by the platform."
            actions={
              all("tenant.manage") && (
                <Button
                  size="sm"
                  onClick={() =>
                    edit(profile, base, "Edit Tenant", [
                      { key: "display_name", label: "Name", required: true },
                    ])
                  }
                >
                  Edit Tenant
                </Button>
              )
            }
          />
          <CardBody>
            <FactList
              className="workspace-facts"
              facts={[
                { label: "Name", value: text(profile.display_name) },
                { label: "Plan", value: text(profile.plan_name) || "—" },
                { label: "Status", value: status(profile.state) },
                {
                  label: "Plan limits: Spaces / Apps / Databases",
                  value: `${text(profile.max_spaces ?? "Unlimited")} / ${text(profile.max_apps ?? "Unlimited")} / ${text(profile.max_databases ?? "Unlimited")}`,
                },
              ]}
            />
          </CardBody>
        </Card>
      )}
      {route.tab === "databases" && (
        <Collection
          path={base + "/database-resources"}
          caption="Databases"
          header={
            <CardHeader
              title="Databases"
              description="Database resources that back this Tenant's Spaces. A Space's binding never changes; retiring a resource preserves its physical data."
            />
          }
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
                      <Button
                        size="sm"
                        variant="ghost"
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
                      </Button>
                    )}
                    {r.management_mode === "registered" &&
                      !["provisioning", "retired"].includes(text(r.state)) && (
                        <Button
                          size="sm"
                          variant="ghost"
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
                        </Button>
                      )}
                    {r.management_mode === "registered" &&
                      r.state === "ready" && (
                        <Button
                          size="sm"
                          variant="ghost"
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
                        </Button>
                      )}
                    {!["provisioning", "retired"].includes(text(r.state)) && (
                      <Button
                        variant="danger"
                        size="sm"
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
                      </Button>
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
