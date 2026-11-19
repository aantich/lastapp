import {
  text,
  type AgentGrant,
  type Deployment,
  type EntityItem,
  type Page,
  type WorkspaceItem,
} from "../api-types";
import { useApiQueries, useApiQuery } from "../query";
import { useDeploymentStates } from "../deployment";
import { useWorkspaceProgress, workspacesPath } from "../workspaces";
import { agentGrantsPath } from "../actors";
import type { TenantScreen } from "./screen";

// The management reads behind the Tenant's Home.
export function useHomeReads(t: TenantScreen) {
  const { all, base, can, overview, scoped, spaces, tenant } = t;
  const loaded = !!overview.tenant;
  const states = useDeploymentStates();
  const recent = overview.recent_apps || [];
  const deployable = recent.filter((a) =>
    scoped("release.manage", text(a.space_id), text(a.id)),
  );
  const deployments = useApiQueries<{ deployment: Deployment | null }>(
    deployable.map((a) => "/v1/apps/" + text(a.id) + "/deployment"),
  );
  const deploymentOf = new Map(
    deployable.map((a, i) => [text(a.id), deployments[i]?.data?.deployment]),
  );
  // A Tenant security manager sees every agent grant in the Tenant; anyone
  // else sees their own grants, narrowed here to this Tenant.
  const manager = all("authorization.manage");
  const grantsQuery = useApiQuery<Page<AgentGrant>>(
    agentGrantsPath(tenant, manager),
  );
  // Measured against the time the grants were read, so a render is pure.
  const now = grantsQuery.dataUpdatedAt;
  const grants = (grantsQuery.data?.items || []).filter(
    (g) =>
      g.tenant_id === tenant &&
      g.state === "active" &&
      new Date(g.expires_at).getTime() > now,
  );
  const activity = useApiQuery<Page>(
    can("audit.read") ? base + "/audit-events?limit=4" : null,
  );
  // The same reads as the sidebar's workspaces and the entity designer's
  // list, so Home adds no request they have already made.
  const progress = useWorkspaceProgress();
  const workspaceSpaces = spaces.filter((sp) =>
    scoped("workspace.manage", text(sp.id), ""),
  );
  const workspaceReads = useApiQueries<Page<WorkspaceItem>>(
    workspaceSpaces.map((sp) => workspacesPath(text(sp.id))),
  );
  const entitySpaces = spaces.filter((sp) =>
    scoped("entity.manage", text(sp.id), ""),
  );
  const entityReads = useApiQueries<{ items: EntityItem[] }>(
    entitySpaces.map((sp) => `/v1/spaces/${text(sp.id)}/entities`),
  );
  return {
    loaded,
    states,
    recent,
    deployable,
    deploymentOf,
    grantsQuery,
    now,
    grants,
    activity,
    progress,
    workspaceSpaces,
    workspaceReads,
    entitySpaces,
    entityReads,
  };
}

export type HomeReads = ReturnType<typeof useHomeReads>;
