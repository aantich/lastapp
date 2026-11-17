import { Collection } from "../ui";
import { text } from "../api-types";
import { status as badge } from "../console-forms";
import { createSpace } from "./forms";
import { nameColumn, statusColumn, type TenantScreen } from "./screen";

// The Tenant's App list, with a first-Space prompt for a new Tenant.
export function AppsPage({ t }: { t: TenantScreen }) {
  const { all, base, openApp, overview } = t;
  return (
    <>
      {overview.spaces === 0 && all("space.manage") && (
        <section className="panel first-space">
          <div>
            <h2>Create your first Space</h2>
            <p>A Space gives your Apps a shared database.</p>
          </div>
          <button className="primary" onClick={() => void createSpace(t)}>
            Create Space
          </button>
        </section>
      )}
      <Collection
        key={base + "apps"}
        path={base + "/apps"}
        columns={[
          nameColumn,
          ["Space", (r) => text(r.space_name)],
          statusColumn,
        ]}
        empty="No Apps are available yet."
        actions={(r) => (
          <button className="text-button" onClick={() => openApp(r)}>
            Open
          </button>
        )}
      />
    </>
  );
}
// The Tenant's Spaces.
export function SpacesPage({ t }: { t: TenantScreen }) {
  const { base, openSpace } = t;
  return (
    <Collection
      key={base + "spaces"}
      path={base + "/spaces"}
      columns={[
        nameColumn,
        ["Apps", (r) => text(r.app_count)],
        ["Database", (r) => badge(r.database_state)],
        statusColumn,
      ]}
      empty="Create a Space to organize your Apps."
      actions={(r) => (
        <button className="text-button" onClick={() => openSpace(r)}>
          Open
        </button>
      )}
    />
  );
}
