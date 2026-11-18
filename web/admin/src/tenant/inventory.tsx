import { FolderPlus } from "lucide-react";
import {
  Button,
  Card,
  Collection,
  EmptyState,
  Stack,
  StatusBadge,
} from "../ui";
import { text } from "../api-types";
import { createSpace } from "./forms";
import {
  createdColumn,
  nameColumn,
  openAction,
  statusColumn,
  type TenantScreen,
} from "./screen";

// The Tenant's App list, with a first-Space prompt for a new Tenant.
export function AppsPage({ t }: { t: TenantScreen }) {
  const { all, base, openApp, overview } = t;
  return (
    <Stack>
      {overview.spaces === 0 && all("space.manage") && (
        <Card>
          <EmptyState
            icon={<FolderPlus size={28} aria-hidden />}
            title="Create your first Space"
            actions={
              <Button variant="primary" onClick={() => void createSpace(t)}>
                Create Space
              </Button>
            }
          >
            A Space gives your Apps a shared database.
          </EmptyState>
        </Card>
      )}
      <Collection
        key={base + "apps"}
        path={base + "/apps"}
        caption="Apps"
        columns={[
          nameColumn,
          ["Space", (r) => text(r.space_name)],
          ["Micro-apps", (r) => text(r.microapp_count)],
          statusColumn,
          createdColumn,
        ]}
        empty="No Apps are available yet."
        actions={openAction(openApp)}
      />
    </Stack>
  );
}

// The Tenant's Spaces.
export function SpacesPage({ t }: { t: TenantScreen }) {
  const { base, openSpace } = t;
  return (
    <Collection
      key={base + "spaces"}
      path={base + "/spaces"}
      caption="Spaces"
      columns={[
        nameColumn,
        ["Apps", (r) => text(r.app_count)],
        ["Database", (r) => <StatusBadge state={r.database_state} />],
        ["Layout", (r) => text(r.database_layout)],
        statusColumn,
      ]}
      empty="Create a Space to organize your Apps."
      actions={openAction(openSpace)}
    />
  );
}
