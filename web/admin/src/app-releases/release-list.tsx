import { ago } from "../actors";
import {
  Button,
  CellActions,
  CellTitle,
  RowTable,
  StatusBadge,
  type RowTableRow,
} from "../ui";
import type { Deployment, ReleaseItem } from "../api-types";

// The App's releases, each with its place relative to production.
export function ReleaseList({
  listed,
  deployment,
  liveState,
  live,
  failed,
  development,
  serving,
  busy,
  loaded,
  hasMore,
  loadMore,
  preview,
  open,
}: {
  listed: ReleaseItem[];
  deployment: Deployment | null | undefined;
  liveState: string;
  live: boolean;
  failed: boolean;
  development: boolean;
  serving: string | undefined;
  busy: boolean;
  loaded: boolean;
  hasMore: boolean;
  loadMore: () => void;
  preview: (release: string) => void;
  open: (release: string) => void;
}) {
  // A release's place: what production serves, what is deploying or
  // failed, earlier production releases, and in a workspace what preview
  // serves.
  function chip(r: ReleaseItem) {
    if (deployment?.release_id === r.id && !live)
      return failed ? (
        <StatusBadge state={deployment.state} label="Failed" />
      ) : (
        <StatusBadge
          state={deployment.state}
          tone="warning"
          label="Deploying"
        />
      );
    if (development)
      return deployment?.release_id === r.id ? (
        <StatusBadge state={liveState} label="Production" />
      ) : serving === r.id ? (
        <StatusBadge state="preview" tone="info" label="In preview" />
      ) : null;
    return serving === r.id ? (
      <StatusBadge state={liveState} label="Production" />
    ) : (
      <StatusBadge state="previous" tone="neutral" label="Previous" />
    );
  }
  const rows: RowTableRow[] = listed.map((r) => {
    const author = r.agent_client_name
      ? `${r.agent_client_name} for ${r.created_by_name}`
      : r.created_by_name;
    return {
      id: r.id,
      cells: [
        <CellTitle
          title={r.display_name}
          meta={[ago(r.created_at), author].filter(Boolean).join(" · ")}
        />,
        chip(r),
        <CellActions>
          {development && (
            <Button
              size="sm"
              disabled={busy}
              aria-label={`Preview ${r.display_name}`}
              onClick={() => preview(r.id)}
            >
              Preview
            </Button>
          )}
          <Button
            size="sm"
            disabled={busy}
            aria-label={`${r.display_name} details`}
            onClick={() => open(r.id)}
          >
            Details
          </Button>
        </CellActions>,
      ],
    };
  });
  return (
    <RowTable
      className="release-list"
      rowClassName="release-item"
      label="Releases"
      title="Releases"
      note="Immutable build + micro-app pins + Space snapshot"
      columns="minmax(var(--ui-measure-sm), 2fr) var(--ui-col-time) auto"
      rows={rows}
      empty={loaded ? "No releases yet." : "Loading…"}
      footer={
        hasMore && (
          <Button size="sm" onClick={loadMore}>
            Load more
          </Button>
        )
      }
    />
  );
}
