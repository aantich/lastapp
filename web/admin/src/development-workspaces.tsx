import { useState } from "react";
import { ExternalLink, Plus, RefreshCw } from "lucide-react";
import { api } from "./api";
import {
  Alert,
  Badge,
  Button,
  CardHeader,
  Collection,
  confirmAction,
  FormModal,
  IconButton,
  InlineError,
  Stack,
  StatusBadge,
  statusTone,
  toast,
  type Modal,
} from "./ui";
import { num, text, type Item } from "./api-types";
import { useAction } from "./use-action";
import { refreshQueries, useApiQuery } from "./query";

// A Space's Development tab: the signed-in author's isolated workspaces (an
// agent's own workspaces included). Each copies the published Space
// configuration into its own database; the list refreshes itself while one
// is still being prepared (a work-in-progress state, statusTone "warning").
export function DevelopmentWorkspaces({
  space,
  openSpace,
}: {
  space: string;
  openSpace: (id: string) => void;
}) {
  const [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState("");
  const { busy, run } = useAction(space, {
    onSuccess: () => {},
    onError: setError,
  });
  const change = (row: Item, retry: boolean) =>
    api(
      `/v1/workspaces/${row.id}/${retry ? "retry" : "archive"}`,
      "POST",
      {},
      num(row.revision),
    );
  async function archive(row: Item) {
    const confirmed = await confirmAction({
      title: `Archive ${text(row.display_name)}?`,
      message:
        "Stops its development APIs and previews. Production and stored data are preserved.",
      confirmLabel: "Archive workspace",
      danger: true,
    });
    if (!confirmed) return;
    await run(async () => {
      await change(row, false);
      toast(`${text(row.display_name)} archived`, { tone: "success" });
    });
  }
  return (
    <Stack>
      {error && <Alert>{error}</Alert>}
      <Collection
        key={space}
        path={`/v1/spaces/${space}/workspaces`}
        caption="Development workspaces"
        header={
          <CardHeader
            title="Development workspaces"
            description="Work privately against an empty database. Production stays separate."
            actions={
              <>
                <IconButton
                  label="Refresh workspaces"
                  icon={<RefreshCw size={16} aria-hidden />}
                  tooltip="bottom"
                  onClick={() => void refreshQueries()}
                />
                <Button
                  variant="primary"
                  icon={<Plus size={16} aria-hidden />}
                  onClick={() =>
                    setModal({
                      title: "Create development workspace",
                      description:
                        "Copies the published Space configuration into a new, isolated database. Records, role assignments and approved App origins are excluded.",
                      success: "Workspace created; preparing its database",
                      fields: [
                        {
                          key: "display_name",
                          label: "Workspace name",
                          required: true,
                        },
                      ],
                      initial: { display_name: "My workspace" },
                      submit: "Create workspace",
                      save: (v) =>
                        api(`/v1/spaces/${space}/workspaces`, "POST", {
                          id: crypto.randomUUID(),
                          display_name: v.display_name,
                        }),
                    })
                  }
                >
                  Create workspace
                </Button>
              </>
            }
          />
        }
        poll={(rows) =>
          rows.some((r) => statusTone(r.state) === "warning") ? 2000 : false
        }
        columns={[
          [
            "Workspace",
            (r) => (
              <Stack gap="sm">
                <strong>{text(r.display_name)}</strong>
                {!!r.owner_agent_grant_id && (
                  <span>
                    <Badge tone="info">Agent workspace</Badge>
                  </span>
                )}
                {text(r.error_message) && (
                  <InlineError>{text(r.error_message)}</InlineError>
                )}
              </Stack>
            ),
          ],
          ["Status", (r) => <StatusBadge state={r.state} />],
          ["Created", (r) => new Date(text(r.created_at)).toLocaleString()],
        ]}
        empty="Create a workspace here, or let your connected agent create its own."
        actions={(r) => (
          <>
            {r.state === "ready" && (
              <Button
                size="sm"
                variant="primary"
                onClick={() => openSpace(text(r.backing_space_id))}
              >
                Open workspace
              </Button>
            )}
            {r.state === "failed" && (
              <Button
                size="sm"
                disabled={busy}
                onClick={() => void run(() => change(r, true))}
              >
                Retry
              </Button>
            )}
            {r.state !== "archived" && (
              <Button
                size="sm"
                variant="danger"
                disabled={busy}
                onClick={() => void archive(r)}
              >
                Archive
              </Button>
            )}
          </>
        )}
      />
      <Alert tone="info" live={false}>
        Each connected agent creates its own workspace through MCP. Source and
        compilation stay on the author’s computer.
      </Alert>
      {modal && (
        <FormModal
          modal={modal}
          close={() => setModal(null)}
          done={() => setModal(null)}
          lookups={{}}
        />
      )}
    </Stack>
  );
}

// Marks every screen of a workspace's backing Space or App as development.
export function DevelopmentBanner({
  workspace,
  openSpace,
}: {
  workspace: string;
  openSpace: (id: string) => void;
}) {
  // The banner is decoration: without workspace details it still marks the
  // screen as development, so a failed read shows no error.
  const { data: info } = useApiQuery<Item>("/v1/workspaces/" + workspace);
  return (
    <Alert
      tone="info"
      live={false}
      className="development-banner"
      title={`Development workspace${info ? ` · ${text(info.display_name)}` : ""}`}
      actions={
        info && (
          <Button
            size="sm"
            icon={<ExternalLink size={14} aria-hidden />}
            onClick={() => openSpace(text(info.production_space_id))}
          >
            Production Space
          </Button>
        )
      }
    >
      Changes and test records are isolated from production.
    </Alert>
  );
}
