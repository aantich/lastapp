import { AgentAccess } from "../agent-access";
import { Button, Card, CardBody, CardHeader, FactList } from "../ui";
import { text } from "../api-types";
import { status } from "../console-forms";
import { TenantDatabases } from "./databases-settings";
import type { TenantScreen } from "./screen";

// Tenant settings: General, Databases and Agent access.
export function TenantSettings({ t }: { t: TenantScreen }) {
  const { all, base, edit, me, profile, route, tenant } = t;
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
      {route.tab === "databases" && <TenantDatabases t={t} />}
    </>
  );
}
