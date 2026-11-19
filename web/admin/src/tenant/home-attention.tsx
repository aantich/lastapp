import type { ReactNode } from "react";
import { AttentionCard, Button, Progress } from "../ui";
import { text } from "../api-types";
import { ago } from "../actors";
import { createSpace } from "./forms";
import type { TenantScreen } from "./screen";
import type { HomeReads } from "./home-reads";
import { deploymentAttention } from "./home-deploy-attention";

const day = 24 * 60 * 60 * 1000;

// What needs a person now, in Home's order: a first Space, deployments,
// workspaces being prepared, unpublished entity changes, databases being
// provisioned or failed, and agent access about to expire.
export function homeAttention(t: TenantScreen, reads: HomeReads) {
  const { all, go, overview } = t;
  const {
    loaded,
    workspaceSpaces,
    workspaceReads,
    progress,
    entitySpaces,
    entityReads,
    grants,
    now,
  } = reads;

  const attention: ReactNode[] = [];
  if (loaded && overview.spaces === 0 && all("space.manage"))
    attention.push(
      <AttentionCard
        key="first-space"
        state="new"
        tone="info"
        stateLabel="Start here"
        title="Create your first Space"
        actions={
          <Button
            size="sm"
            variant="primary"
            onClick={() => void createSpace(t)}
          >
            Create Space
          </Button>
        }
      >
        A Space gives your Apps a shared database, entities and permissions.
      </AttentionCard>,
    );
  attention.push(...deploymentAttention(t, reads));
  workspaceSpaces.forEach((sp, i) => {
    for (const w of workspaceReads[i]?.data?.items || []) {
      const step = w.state === "preparing" && progress(w.preparation_step);
      if (!step) continue;
      attention.push(
        <AttentionCard
          key={"workspace" + w.id}
          state={w.state}
          stateLabel="Preparing"
          tone="warning"
          meta={
            w.agent_client_name ? `by ${w.agent_client_name} · agent` : "by you"
          }
          title={`Workspace ${w.display_name}`}
          extra={<Progress {...step} />}
        >
          {`Copying ${text(sp.display_name)}'s published configuration into a fresh database.`}
        </AttentionCard>,
      );
    }
  });
  entitySpaces.forEach((sp, i) => {
    for (const e of entityReads[i]?.data?.items || []) {
      if (
        !e.has_draft_changes ||
        e.pending_version_id ||
        e.state === "archived"
      )
        continue;
      attention.push(
        <AttentionCard
          key={"entity" + e.id}
          state="draft"
          stateLabel="Review"
          tone="info"
          meta={text(sp.display_name)}
          title={`${e.label} · Unpublished changes`}
          actions={
            <Button size="sm" onClick={() => t.openEntity(text(sp.id), e.id)}>
              Open
            </Button>
          }
        >
          {e.active_version_id
            ? "Its draft differs from the published version. Apps keep using the published version until you publish."
            : "Not published yet. Apps can use it once you publish."}
        </AttentionCard>,
      );
    }
  });
  for (const db of overview.attention || []) {
    const failed = db.state === "failed";
    attention.push(
      <AttentionCard
        key={"db" + text(db.id)}
        state={text(db.state)}
        stateLabel={failed ? "Failed" : "Preparing"}
        meta="Database"
        title={
          failed
            ? `Database ${text(db.database_name)} failed`
            : `Database ${text(db.database_name)} is being prepared`
        }
        actions={
          <Button
            size="sm"
            onClick={() => go("settings", { tab: "databases" })}
          >
            Open databases
          </Button>
        }
      >
        {failed
          ? text(db.last_error_message) || "Provisioning failed."
          : "Spaces that use it can open once it is ready."}
      </AttentionCard>,
    );
  }
  for (const g of grants) {
    const left = new Date(g.expires_at).getTime() - now;
    if (left > 7 * day) continue;
    attention.push(
      <AttentionCard
        key={"grant" + g.id}
        state="expiring"
        tone="warning"
        stateLabel="Expiring"
        meta={`Agent · ${g.owner_name}`}
        title={`${g.client_name} access ends ${ago(g.expires_at)}`}
        actions={
          <Button size="sm" onClick={() => go("agents")}>
            Review agent access
          </Button>
        }
      >
        {`Scoped to ${g.scope_name}. Approve a new connection to keep it working.`}
      </AttentionCard>,
    );
  }
  return attention;
}
