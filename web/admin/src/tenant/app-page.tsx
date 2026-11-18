import { Pencil } from "lucide-react";
import { AppReleases } from "../app-releases";
import { AppDataAccess } from "../entity-designer/app-access";
import {
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  Collection,
  Disclosure,
  FactList,
  Stack,
  StatusBadge,
} from "../ui";
import { AccessControl } from "../access-control";
import { refreshQueries } from "../query";
import { keyOf, optText, text } from "../api-types";
import {
  computeField,
  editFields,
  nameColumn,
  states,
  statusColumn,
  type TenantScreen,
} from "./screen";

// An App's tabs: Micro-apps, Releases, Access and Settings.
export function AppPage({ t }: { t: TenantScreen }) {
  const { app, base, caps, edit, go, profile, route, scoped, tenant } = t;
  const refresh = () => void refreshQueries();
  return (
    <>
      {route.tab === "releases" && (
        <AppReleases
          key={keyOf(app.id)}
          app={app}
          tenant={tenant}
          refresh={refresh}
          openWorkspaces={() =>
            go("space", { space: optText(app.space_id), tab: "development" })
          }
        />
      )}
      {route.tab === "microapps" && (
        <Collection
          key={app.id + "microapps"}
          path={base + "/microapps?app_id=" + app.id}
          caption={`${text(app.display_name)} micro-apps`}
          columns={[
            ["Order", (r) => text(r.navigation_order)],
            nameColumn,
            [
              "Artifact",
              (r) =>
                optText(r.artifact_ref) ? (
                  <Badge mono>{text(r.artifact_ref)}</Badge>
                ) : (
                  "—"
                ),
            ],
            statusColumn,
          ]}
          empty="Add your first micro-app."
          actions={
            scoped("microapp.manage")
              ? (r) => (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      edit(r, base + "/microapps/" + r.id, "Edit micro-app", [
                        {
                          key: "display_name",
                          label: "Name",
                          required: true,
                        },
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
                )
              : undefined
          }
        />
      )}
      {route.tab === "access" && (
        <AccessControl
          key={keyOf(app.id)}
          tenant={tenant}
          caps={caps}
          scopeKind="app"
          scopeId={text(app.id)}
        />
      )}
      {route.tab === "settings" && (
        <Stack>
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
          {scoped("app.manage") && (
            <AppDataAccess
              key={keyOf(app.id)}
              app={text(app.id)}
              spaceManager={scoped("entity.manage", optText(app.space_id), "")}
            />
          )}
        </Stack>
      )}
    </>
  );
}
