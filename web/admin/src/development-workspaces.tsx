import { useState } from "react";
import { Plus, RefreshCw, ExternalLink } from "lucide-react";
import { api } from "./api";
import { Collection } from "./collection";
import { FormModal, type Modal } from "./form-modal";
import { status } from "./console-forms";
import type { Item } from "./api-types";
import "./development-workspaces.css";
import { useAction } from "./use-action";
import { refreshQueries, useApiQuery } from "./query";

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
      row.revision,
    );
  return (
    <section className="development-page">
      <div className="development-heading">
        <div>
          <h2>Development workspaces</h2>
          <p>
            Work privately against an empty database. Production stays separate.
          </p>
        </div>
        <div className="development-actions">
          <button
            aria-label="Refresh workspaces"
            onClick={() => void refreshQueries()}
          >
            <RefreshCw size={16} />
          </button>
          <button
            className="primary"
            onClick={() =>
              setModal({
                title: "Create development workspace",
                description:
                  "Copies the published Space configuration into a new, isolated database. Records, role assignments and approved App origins are excluded.",
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
            <Plus size={16} />
            Create workspace
          </button>
        </div>
      </div>
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      <Collection
        key={space}
        path={`/v1/spaces/${space}/workspaces`}
        columns={[
          [
            "Workspace",
            (r) => (
              <div>
                <strong>{r.display_name}</strong>
                {r.owner_agent_grant_id && (
                  <small className="muted">Agent workspace</small>
                )}
                {r.error_message && <p className="error">{r.error_message}</p>}
              </div>
            ),
          ],
          ["Status", (r) => status(r.state)],
        ]}
        empty="Create a workspace here, or let your connected agent create its own."
        actions={(r) => (
          <div className="development-actions">
            {r.state === "ready" && (
              <button
                className="text-button"
                onClick={() => openSpace(r.backing_space_id)}
              >
                Open workspace
              </button>
            )}
            {r.state === "failed" && (
              <button
                disabled={busy}
                onClick={() => void run(() => change(r, true))}
              >
                Retry
              </button>
            )}
            {r.state !== "archived" && (
              <button
                className="text-button"
                onClick={() =>
                  setModal({
                    title: "Archive workspace",
                    description:
                      "Stops its development APIs and previews. Production and stored data are preserved.",
                    initial: {},
                    fields: [],
                    submit: "Archive workspace",
                    save: () => change(r, false),
                  })
                }
              >
                Archive
              </button>
            )}
          </div>
        )}
      />
      <p className="muted">
        Each connected agent creates its own workspace through MCP. Source and
        compilation stay on the author’s computer.
      </p>
      {modal && (
        <FormModal
          modal={modal}
          close={() => setModal(null)}
          done={() => setModal(null)}
          lookups={{}}
        />
      )}
    </section>
  );
}
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
    <aside className="development-banner">
      <div>
        <strong>
          Development workspace{info ? ` · ${info.display_name}` : ""}
        </strong>
        <span>Changes and test records are isolated from production.</span>
      </div>
      {info && (
        <button onClick={() => openSpace(info.production_space_id)}>
          Production Space <ExternalLink size={14} />
        </button>
      )}
    </aside>
  );
}
