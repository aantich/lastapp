import { Suspense, type ComponentProps } from "react";
import { retryableLazy } from "../retryable-lazy";
import type { EntityDesigner as Designer } from "../entity-designer/designer";
import { DevelopmentWorkspaces } from "../development-workspaces";
import { Collection } from "../collection";
import { AccessControl } from "../access-control";
import { status as badge } from "../console-forms";
import { keyOf, text } from "../api-types";
import { nameColumn, statusColumn, type TenantScreen } from "./screen";

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
          columns={[
            nameColumn,
            ["Micro-apps", (r) => text(r.microapp_count)],
            statusColumn,
          ]}
          actions={(r) => (
            <button className="text-button" onClick={() => openApp(r)}>
              Open
            </button>
          )}
        />
      )}
      {route.tab === "entities" && scoped("entity.manage") && (
        <Suspense fallback={<p>Loading entity designer…</p>}>
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
        <>
          <section className="panel workspace-section">
            <div className="settings-heading">
              <div className="settings-summary">
                <h2>General</h2>
                {badge(space.state)}
              </div>
              {scoped("space.manage") && (
                <button
                  className="secondary"
                  onClick={() =>
                    edit(space, base + "/spaces/" + space.id, "Edit Space")
                  }
                >
                  Edit Space
                </button>
              )}
            </div>
          </section>
          <section className="panel workspace-section">
            <h2 className="section-title">Database</h2>
            <dl className="workspace-facts">
              <dt>Connection</dt>
              <dd>{badge(space.database_state)}</dd>
              <dt>Layout</dt>
              <dd>{text(space.database_layout)}</dd>
            </dl>
            {all("database.read") && (
              <button
                className="secondary"
                onClick={() => go("settings", { tab: "databases" })}
              >
                Manage databases
              </button>
            )}
            <details>
              <summary>Advanced</summary>
              <p>Database: {text(space.database_name)}</p>
              <p>Slug: {text(space.slug)}</p>
            </details>
          </section>
        </>
      )}
    </>
  );
}
