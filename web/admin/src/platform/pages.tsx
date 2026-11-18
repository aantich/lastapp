import { api } from "../api";
import { num, text } from "../api-types";
import {
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  Checkbox,
  Collection,
  FactList,
  Stack,
  StatCard,
  StatGrid,
  Tabs,
  confirmAction,
} from "../ui";
import { useBuiltinRoles } from "../console-config";
import { status } from "../console-forms";
import { addToTenant, editTenant, planForm, serverForm } from "./forms";
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
// Every Tenant on the platform.
export function TenantsPage({ p }: { p: PlatformScreen }) {
  const { go } = p;
  return (
    <Collection
      path="/admin/v1/tenants"
      caption="Tenants"
      columns={[
        columnsName,
        ["Plan", (r) => text(r.plan_name)],
        ["Spaces", (r) => text(r.space_count)],
        ["Apps", (r) => text(r.app_count)],
        columnsStatus,
      ]}
      actions={(r) => (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => go("tenants", text(r.id), "overview")}
        >
          Open
        </Button>
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
      caption="Accounts"
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
            <Checkbox
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
            <Button size="sm" variant="ghost" onClick={() => addToTenant(p, r)}>
              Add to Tenant
            </Button>
          )}
          <Button
            size="sm"
            variant={r.state === "active" ? "danger" : "secondary"}
            disabled={busy}
            onClick={async () => {
              const disable = r.state === "active";
              if (
                disable &&
                !(await confirmAction({
                  title: "Disable account?",
                  message: `${text(r.display_name)} can no longer sign in and their sessions end. You can enable the account again later.`,
                  confirmLabel: "Disable account",
                  danger: true,
                }))
              )
                return;
              act(() =>
                api(
                  "/admin/v1/users/" + r.id,
                  "PATCH",
                  { state: disable ? "disabled" : "active" },
                  num(r.revision),
                ),
              );
            }}
          >
            {r.state === "active" ? "Disable account" : "Enable account"}
          </Button>
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
    <Stack>
      <Tabs
        label="Platform settings"
        className="workspace-tabs"
        items={[
          { id: "servers", label: "Database servers" },
          { id: "plans", label: "Plans" },
        ]}
        value={settingsTab}
        onChange={(id) => go("settings", id)}
      />
      {settingsTab === "plans" ? (
        <Collection
          path="/admin/v1/plans"
          caption="Plans"
          header={
            <CardHeader
              title="Plans"
              description="Limits a Tenant's Spaces, Apps and databases. Empty means unlimited."
            />
          }
          columns={[
            columnsName,
            [
              "Limits",
              (r) =>
                `${r.max_spaces ?? "∞"} Spaces · ${r.max_apps ?? "∞"} Apps · ${r.max_databases ?? "∞"} databases`,
            ],
          ]}
          actions={(r) => (
            <Button size="sm" variant="ghost" onClick={() => planForm(p, r)}>
              Edit plan
            </Button>
          )}
        />
      ) : (
        <Collection
          path="/admin/v1/database-targets"
          caption="Database servers"
          header={
            <CardHeader
              title="Database servers"
              description="MariaDB servers that host Space databases."
            />
          }
          columns={[
            columnsName,
            ["Endpoint", (r) => r.canonical_host + ":" + r.port],
            columnsStatus,
          ]}
          actions={(r) => (
            <Button size="sm" variant="ghost" onClick={() => serverForm(p, r)}>
              Edit server
            </Button>
          )}
        />
      )}
    </Stack>
  );
}
