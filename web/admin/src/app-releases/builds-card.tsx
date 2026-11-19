import { Button, Card, CardHeader, DataTable } from "../ui";
import type { Build } from "../api-types";
import { when } from "./release-dates";

// A development App's uploaded builds; each can become a release.
export function BuildsCard({
  builds,
  busy,
  capture,
}: {
  builds: Build[];
  busy: boolean;
  capture: (build: string) => void;
}) {
  return (
    <Card>
      <CardHeader
        title="Uploaded builds"
        description="Create a release from an earlier upload."
      />
      <DataTable
        caption="Uploaded builds"
        rows={builds}
        rowKey={(b) => b.id}
        columns={[
          {
            key: "build",
            header: "Build",
            render: (b) => <code>{b.content_hash.slice(0, 12)}</code>,
          },
          {
            key: "size",
            header: "Size",
            numeric: true,
            render: (b) => `${Math.ceil(b.size_bytes / 1024)} KB`,
          },
          {
            key: "uploaded",
            header: "Uploaded",
            render: (b) => when(b.created_at),
          },
        ]}
        actions={(b) => (
          <Button size="sm" disabled={busy} onClick={() => capture(b.id)}>
            Create release
          </Button>
        )}
      />
    </Card>
  );
}
