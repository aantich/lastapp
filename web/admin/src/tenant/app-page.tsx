import { ArrowUpRight, Pencil } from "lucide-react";
import { AppReleases } from "../app-releases";
import { AppDataAccess } from "../entity-designer/app-access";
import {
  AppHeader,
  Badge,
  Button,
  Card,
  CellActions,
  CellMono,
  CardBody,
  CardHeader,
  CardRow,
  Disclosure,
  FactList,
  Inline,
  LinkButton,
  PageStack,
  RowTable,
  Stack,
  StatusBadge,
  Tabs,
  TitledCard,
  ValueBox,
} from "../ui";
import { AccessControl } from "../access-control";
import { refreshQueries, useApiQuery } from "../query";
import {
  isItem,
  items,
  keyOf,
  optText,
  text,
  type Item,
  type Page,
} from "../api-types";
import { useAppDeployment, useDeploymentStates } from "../deployment";
import { appStatus, hostOf } from "./app-status";
import { tenantTabs } from "./navigation";
import { createMicro } from "./forms";
import { computeField, editFields, states, type TenantScreen } from "./screen";

// An App: its header (tile, name, hosted address, status, launch) over its
// sections: Releases, Micro-apps and Data access, then Access and Settings.
// The status and address come from the App read and, where this account may
// manage releases, its latest deployment.
export function AppPage({ t }: { t: TenantScreen }) {
  const { all, app, can, go, route, scoped, tenant } = t;
  const deploymentStates = useDeploymentStates();
  const { deployment, error: deploymentError } = useAppDeployment(
    text(app.id),
    scoped("release.manage"),
  );
  const status = appStatus(app, deployment, deploymentStates);
  const tabs = tenantTabs("app", { all, can, scoped });
  const tab = tabs.some((x) => x.id === route.tab)
    ? route.tab
    : tabs[0]?.id || "";
  const origin = text(app.hosted_origin);
  return (
    <PageStack>
      <AppHeader
        name={text(app.display_name)}
        tone={status.tone}
        address={hostOf(origin)}
        status={status.badge}
        actions={
          <>
            {tab === "microapps" && scoped("microapp.manage") && (
              <Button onClick={() => createMicro(t)}>Add micro-app</Button>
            )}
            {!app.workspace_id &&
              scoped("workspace.manage", optText(app.space_id), "") && (
                <Button
                  onClick={() =>
                    go("space", {
                      space: optText(app.space_id),
                      tab: "development",
                    })
                  }
                >
                  Development workspaces
                </Button>
              )}
            {optText(app.active_release_id) && origin && (
              <LinkButton
                href={origin + "/"}
                external
                variant="primary"
                trailingIcon={<ArrowUpRight size={16} aria-hidden />}
              >
                Launch app
              </LinkButton>
            )}
          </>
        }
      />
      <Tabs
        label={`${text(app.display_name)} sections`}
        variant="line"
        className="workspace-tabs"
        items={tabs}
        value={tab}
        onChange={(id) => go("app", { app: route.app, tab: id })}
      />
      {tab === "releases" && (
        <AppReleases
          key={keyOf(app.id)}
          app={app}
          tenant={tenant}
          deployment={deployment}
          deploymentError={deploymentError}
          refresh={() => void refreshQueries()}
        />
      )}
      {tab === "microapps" && <MicroApps t={t} />}
      {tab === "data" && <DataAccess t={t} />}
      {tab === "access" && (
        <AccessControl
          key={keyOf(app.id)}
          tenant={tenant}
          caps={t.caps}
          scopeKind="app"
          scopeId={text(app.id)}
        />
      )}
      {tab === "settings" && <Settings t={t} />}
    </PageStack>
  );
}

// The App's micro-apps in navigation order. The artifact reference is
// Tenant-authored metadata, shown as text and never fetched.
function MicroApps({ t }: { t: TenantScreen }) {
  const { app, base, edit, scoped } = t;
  const list = useApiQuery<Page>(
    `${base}/microapps?app_id=${text(app.id)}&limit=200`,
  );
  const manage = scoped("microapp.manage");
  return (
    <RowTable
      label={`${text(app.display_name)} micro-apps`}
      columns={
        "40px minmax(180px, 2fr) minmax(160px, 2fr) 100px" +
        (manage ? " auto" : "")
      }
      empty={
        list.error
          ? list.error.message
          : list.data
            ? "No micro-apps yet."
            : "Loading…"
      }
      rows={(list.data?.items || []).map((m) => ({
        id: text(m.id),
        cells: [
          <CellMono>{text(m.navigation_order)}</CellMono>,
          <strong>{text(m.display_name)}</strong>,
          <CellMono>{text(m.artifact_ref)}</CellMono>,
          <StatusBadge state={m.state} />,
          ...(manage
            ? [
                <CellActions>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      edit(m, base + "/microapps/" + m.id, "Edit micro-app", [
                        { key: "display_name", label: "Name", required: true },
                        {
                          key: "state",
                          label: "Status",
                          type: "select",
                          options: states,
                          required: true,
                        },
                        {
                          key: "artifact_ref",
                          label: "Artifact reference",
                          advanced: true,
                        },
                        {
                          key: "navigation_order",
                          label: "Navigation order",
                          type: "number",
                          advanced: true,
                        },
                      ])
                    }
                  >
                    Edit micro-app
                  </Button>
                </CellActions>,
              ]
            : []),
        ],
      }))}
      footer={
        list.data?.pagination?.has_more && (
          <CellMono>
            Showing the first {list.data.items.length} micro-apps.
          </CellMono>
        )
      }
    />
  );
}

// The App's data access: the one approved origin and the ceiling of Space
// permission paths it may use (each with its fields), then the editor.
function DataAccess({ t }: { t: TenantScreen }) {
  const { app, scoped } = t;
  const id = text(app.id);
  const state = useApiQuery<Item>(`/v1/apps/${id}/data-access`, {
    refetchInterval: false,
  }).data;
  const contracts = useApiQuery<Item>(`/v1/apps/${id}/entity-contracts`, {
    refetchInterval: false,
  }).data;
  const access = isItem(state?.access) ? state.access : undefined;
  const paths = items(contracts?.entities).flatMap((c) =>
    isItem(c.definition)
      ? items(c.definition.permissions).map((path) => ({
          path,
          entity: text((c.definition as Item).label),
        }))
      : [],
  );
  const names = (v: unknown) =>
    Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
  // Each gate keeps its fields: the gate's own narrowing, else every field
  // the Space permission reaches.
  const ceiling = items(access?.ceiling).map((g) => {
    const p = paths.find((x) => x.path.id === g.permission);
    const own = names(g.fields);
    return {
      key: text(g.permission),
      label: p ? `${p.entity} · ${text(p.path.label)}` : text(g.permission),
      fields: own.length ? own : names(p?.path.fields),
    };
  });
  return (
    <Stack>
      <CardRow>
        <TitledCard
          layout="stacked"
          title="Approved origins"
          note="Browsers allowed to sign in to this App’s data API."
        >
          {!access ? null : text(access.origin) ? (
            <ValueBox>{text(access.origin)}</ValueBox>
          ) : (
            <ValueBox>No origin approved yet</ValueBox>
          )}
        </TitledCard>
        <TitledCard
          layout="stacked"
          title="Permission ceiling"
          note="The App can narrow Space access, never widen it."
        >
          {access && (
            <Inline>
              {access.inherit ? (
                <Badge pill>All current and future Space permissions</Badge>
              ) : ceiling.length ? (
                ceiling.map((c) => (
                  <Badge pill key={c.key}>
                    {c.label}
                    {c.fields.length ? ` · ${c.fields.join(", ")}` : ""}
                  </Badge>
                ))
              ) : (
                <Badge pill>No Space permissions</Badge>
              )}
            </Inline>
          )}
        </TitledCard>
      </CardRow>
      {scoped("app.manage") && (
        <Card>
          <Disclosure summary="Edit data access">
            <AppDataAccess
              key={id}
              app={id}
              spaceManager={scoped("entity.manage", optText(app.space_id), "")}
            />
          </Disclosure>
        </Card>
      )}
    </Stack>
  );
}

// The App's name, status and compute preference.
function Settings({ t }: { t: TenantScreen }) {
  const { app, base, edit, profile, scoped } = t;
  return (
    <Card>
      <CardHeader
        title="General"
        actions={
          <>
            <StatusBadge state={app.state} />
            {scoped("app.manage") && (
              <Button
                icon={<Pencil size={14} aria-hidden />}
                onClick={() =>
                  edit(app, base + "/apps/" + app.id, "Edit App", [
                    ...editFields,
                    computeField(
                      !!profile.allow_dedicated_compute ||
                        app.compute_mode === "dedicated",
                    ),
                  ])
                }
              >
                Edit App
              </Button>
            )}
          </>
        }
      />
      <CardBody>
        <FactList
          facts={[
            { label: "Name", value: text(app.display_name) },
            { label: "Space", value: text(app.space_name) },
          ]}
        />
        <Disclosure summary="Advanced">
          <FactList
            facts={[
              { label: "Slug", value: text(app.slug), mono: true },
              {
                label: "Compute preference",
                value: text(app.compute_mode),
              },
            ]}
          />
        </Disclosure>
      </CardBody>
    </Card>
  );
}
