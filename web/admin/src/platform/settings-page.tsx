import { Button, CardHeader, Collection, Stack, Tabs } from "../ui";
import { planForm } from "./plan-form";
import { serverForm } from "./server-form";
import { columnsName, columnsStatus, type PlatformScreen } from "./screen";

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
