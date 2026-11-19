import { api } from "../api";
import { num, text } from "../api-types";
import {
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  Collection,
  FactList,
  Stack,
  StatCard,
  StatGrid,
  Tabs,
  confirmAction,
} from "../ui";
import { status } from "../console-forms";
import { editTenant } from "./forms";
import { columnsName, columnsStatus, type PlatformScreen } from "./screen";

// One Tenant's Overview, Administrators and Settings tabs.
export function PlatformTenant({ p }: { p: PlatformScreen }) {
  const { act, busy, go, overview, profile, route, tenant } = p;
  const { tab } = route;
  return (
    <Stack>
      <Tabs
        label="Tenant sections"
        className="workspace-tabs"
        items={["overview", "administrators", "settings"].map((id) => ({
          id,
          label: id[0].toUpperCase() + id.slice(1),
        }))}
        value={tab}
        onChange={(id) => go("tenants", tenant, id)}
      />
      {tab === "overview" && (
        <>
          <StatGrid>
            <StatCard label="Spaces" value={text(overview.spaces ?? "—")} />
            <StatCard label="Apps" value={text(overview.apps ?? "—")} />
            <StatCard
              label="Databases"
              value={text(overview.databases ?? "—")}
            />
          </StatGrid>
          <Card>
            <CardHeader
              title="Tenant"
              description="Plan and lifecycle status, set by the platform."
              actions={
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => go("tenants", tenant, "administrators")}
                >
                  Manage administrators
                </Button>
              }
            />
            <CardBody>
              <FactList
                className="workspace-facts"
                facts={[
                  { label: "Plan", value: text(profile.plan_name) || "—" },
                  { label: "Status", value: status(profile.state) },
                ]}
              />
            </CardBody>
          </Card>
          <Collection
            key={tenant + "inventory"}
            path={`/v1/tenants/${tenant}/apps`}
            caption="Recent Apps"
            header={<CardHeader title="Recent Apps" />}
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
          caption="Administrators"
          header={
            <CardHeader
              title="Administrators"
              description="They manage this Tenant's teams, roles and Space/App managers. Tenant administration grants no automatic access to business records."
            />
          }
          columns={[
            columnsName,
            columnsStatus,
            ["Account", (r) => text(r.user_state)],
          ]}
          actions={(r) => (
            <Button
              variant="danger"
              size="sm"
              disabled={busy}
              onClick={async () => {
                if (
                  await confirmAction({
                    title: "Remove administrator?",
                    message: `${text(r.display_name)} loses administrator access to this Tenant. The account itself stays.`,
                    confirmLabel: "Remove administrator",
                    danger: true,
                  })
                )
                  act(() =>
                    api(
                      `/v1/tenants/${tenant}/role-assignments/${r.id}`,
                      "DELETE",
                      undefined,
                      num(r.revision),
                    ),
                  );
              }}
            >
              Remove administrator
            </Button>
          )}
        />
      )}
      {tab === "settings" && (
        <Card>
          <CardHeader
            title="Tenant settings"
            description="The Tenant's name, plan and status."
          />
          <CardBody>
            <FactList
              facts={[
                { label: "Name", value: text(profile.display_name) || "—" },
                { label: "Plan", value: text(profile.plan_name) || "—" },
                { label: "Status", value: status(profile.state) },
              ]}
            />
          </CardBody>
          <CardFooter>
            <Button variant="link" onClick={() => go("activity", tenant)}>
              View Tenant activity
            </Button>
            <Button onClick={() => editTenant(p)}>Edit Tenant</Button>
          </CardFooter>
        </Card>
      )}
    </Stack>
  );
}
