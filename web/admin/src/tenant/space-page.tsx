import { Suspense, type ComponentProps } from "react";
import { Pencil } from "lucide-react";
import { retryableLazy } from "../retryable-lazy";
import type { EntityDesigner as Designer } from "../entity-designer/designer";
import { DevelopmentWorkspaces } from "../development-workspaces";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Collection,
  Disclosure,
  EmptyState,
  FactList,
  Stack,
  StatusBadge,
} from "../ui";
import { AccessControl } from "../access-control";
import { keyOf, text } from "../api-types";
import {
  createdColumn,
  nameColumn,
  openAction,
  statusColumn,
  type TenantScreen,
} from "./screen";

// The entity designer is the console's largest screen; load it on first use.
const EntityDesigner = retryableLazy<ComponentProps<typeof Designer>>(() =>
  import("../entity-designer/designer").then((m) => ({
    default: m.EntityDesigner,
  })),
);

// A Space's tabs: Apps, Development, Entities, Access and Settings.
export function SpacePage({ t }: { t: TenantScreen }) {
  const {
    all,
    base,
    caps,
    edit,
    go,
    me,
    openApp,
    route,
    scoped,
    space,
    tenant,
  } = t;
  return (
    <>
      {route.tab === "development" && !space.workspace_id && (
        <DevelopmentWorkspaces
          space={text(space.id)}
          openSpace={(id) => go("space", { space: id })}
        />
      )}
      {route.tab === "apps" && (
        <Collection
          key={space.id + "apps"}
          path={base + "/apps?space_id=" + space.id}
          caption={`${text(space.display_name)} Apps`}
          columns={[
            nameColumn,
            ["Micro-apps", (r) => text(r.microapp_count)],
            statusColumn,
            createdColumn,
          ]}
          empty="No Apps in this Space yet."
          actions={openAction(openApp)}
        />
      )}
      {route.tab === "entities" && scoped("entity.manage") && (
        <Suspense
          fallback={
            <Card>
              <EmptyState compact>Loading entity designer…</EmptyState>
            </Card>
          }
        >
          <EntityDesigner
            key={keyOf(space.id)}
            space={text(space.id)}
            tenant={tenant}
            me={me}
          />
        </Suspense>
      )}
      {route.tab === "access" && (
        <AccessControl
          key={keyOf(space.id)}
          tenant={tenant}
          caps={caps}
          scopeKind="space"
          scopeId={text(space.id)}
        />
      )}
      {route.tab === "settings" && (
        <Stack>
          <Card>
            <CardHeader
              title="General"
              actions={
                <>
                  <StatusBadge state={space.state} />
                  {scoped("space.manage") && (
                    <Button
                      icon={<Pencil size={14} aria-hidden />}
                      onClick={() =>
                        edit(space, base + "/spaces/" + space.id, "Edit Space")
                      }
                    >
                      Edit Space
                    </Button>
                  )}
                </>
              }
            />
            <CardBody>
              <FactList
                facts={[
                  { label: "Name", value: text(space.display_name) },
                  { label: "Slug", value: text(space.slug), mono: true },
                ]}
              />
            </CardBody>
          </Card>
          <Card>
            <CardHeader
              title="Database"
              description="The Space's one database binding; it cannot change."
              actions={
                all("database.read") && (
                  <Button onClick={() => go("settings", { tab: "databases" })}>
                    Manage databases
                  </Button>
                )
              }
            />
            <CardBody>
              <FactList
                className="workspace-facts"
                facts={[
                  {
                    label: "Connection",
                    value: <StatusBadge state={space.database_state} />,
                  },
                  { label: "Layout", value: text(space.database_layout) },
                ]}
              />
              <Disclosure summary="Advanced">
                <FactList
                  facts={[
                    {
                      label: "Database",
                      value: text(space.database_name),
                      mono: true,
                    },
                  ]}
                />
              </Disclosure>
            </CardBody>
          </Card>
        </Stack>
      )}
    </>
  );
}
