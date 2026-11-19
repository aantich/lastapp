import { useState } from "react";
import { FolderPlus } from "lucide-react";
import {
  AppTile,
  Button,
  CellMono,
  Card,
  Collection,
  EmptyState,
  PageStack,
  RowTable,
  StatusBadge,
  Icon,
} from "../ui";
import { isItem, text, type DeploymentSummary, type Page } from "../api-types";
import { useApiQuery } from "../query";
import { useDeploymentStates } from "../deployment";
import { appStatus, hostOf } from "./app-status";
import { createSpace } from "./forms";
import {
  nameColumn,
  openAction,
  statusColumn,
  type TenantScreen,
} from "./screen";

// The Tenant's Apps: one row per App with its Space, status and hosted
// address, all from the App list (GET …/apps), which carries each App's
// active release, hosted origin and, where this account may manage its
// releases, its latest deployment. A new Tenant gets a first-Space prompt.
export function AppsPage({ t }: { t: TenantScreen }) {
  const { all, base, openApp, overview } = t;
  const [page, setPage] = useState(1);
  const states = useDeploymentStates();
  const list = useApiQuery<Page>(`${base}/apps?page=${page}`);
  const apps = list.data?.items || [];
  const more = !!list.data?.pagination?.has_more;
  return (
    <PageStack>
      {overview.spaces === 0 && all("space.manage") && (
        <Card>
          <EmptyState
            icon={<Icon icon={FolderPlus} size="6xl" aria-hidden />}
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
      <RowTable
        label="Apps"
        columns="minmax(var(--ui-col-name), 2fr) var(--ui-col-label) var(--ui-col-status) minmax(var(--ui-measure-sm), 2fr)"
        headers={["App", "Space", "Status", "Hostname"]}
        empty={
          list.error
            ? list.error.message
            : list.data
              ? "No Apps are available yet."
              : "Loading…"
        }
        rows={apps.map((a) => {
          const name = text(a.display_name);
          const status = appStatus(
            a,
            isItem(a.latest_deployment)
              ? (a.latest_deployment as DeploymentSummary)
              : undefined,
            states,
          );
          return {
            id: text(a.id),
            onOpen: () => openApp(a),
            cells: [
              <>
                <AppTile name={name} tone={status.tone} />
                <strong>{name}</strong>
              </>,
              text(a.space_name),
              status.badge,
              <CellMono>{hostOf(a.hosted_origin)}</CellMono>,
            ],
          };
        })}
        footer={
          (page > 1 || more) && (
            <>
              <Button
                size="sm"
                disabled={page === 1}
                aria-label="Previous records"
                onClick={() => setPage(page - 1)}
              >
                Previous
              </Button>
              <Button
                size="sm"
                disabled={!more}
                aria-label="Next records"
                onClick={() => setPage(page + 1)}
              >
                Next
              </Button>
            </>
          )
        }
      />
    </PageStack>
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
