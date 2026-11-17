import { AppReleases } from "../app-releases";
import { AppDataAccess } from "../entity-designer/app-access";
import { Collection } from "../collection";
import { AccessControl } from "../access-control";
import { status as badge } from "../console-forms";
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
          columns={[nameColumn, statusColumn]}
          empty="Add your first micro-app."
          actions={
            scoped("microapp.manage")
              ? (r) => (
                  <button
                    className="text-button"
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
                  </button>
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
        <>
          <section className="panel workspace-section">
            <div className="settings-heading">
              <div className="settings-summary">
                <h2>General</h2>
                {badge(app.state)}
              </div>
              {scoped("app.manage") && (
                <button
                  className="secondary"
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
                </button>
              )}
            </div>
            <details>
              <summary>Advanced</summary>
              <p>Slug: {text(app.slug)}</p>
              <p>Compute preference: {text(app.compute_mode)}</p>
            </details>
          </section>
          {scoped("app.manage") && (
            <AppDataAccess
              key={keyOf(app.id)}
              app={text(app.id)}
              spaceManager={scoped("entity.manage", optText(app.space_id), "")}
            />
          )}
        </>
      )}
    </>
  );
}
