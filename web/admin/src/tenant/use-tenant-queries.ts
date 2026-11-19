import { useApiQuery } from "../query";
import { agentGrantsPath } from "../actors";
import type {
  AgentGrant,
  CapabilitiesResponse,
  Item,
  Page,
  TenantOverview,
} from "../api-types";

// What every Tenant workspace page shares: the overview, the capabilities,
// the open Space or App, the sidebar's Spaces and its Agents count. `base` is
// the Tenant's API path and `live` the refresh interval.
export function useTenantQueries(
  tenant: string,
  base: string,
  parsed: { page: string; space: string; app: string },
  live: number | false,
) {
  const overviewQuery = useApiQuery<TenantOverview>(
      tenant ? base + "/overview" : null,
      { refetchInterval: live },
    ),
    capsQuery = useApiQuery<CapabilitiesResponse>(
      tenant ? base + "/capabilities" : null,
      { refetchInterval: live },
    ),
    // A Space or App screen also reads its subject.
    spaceQuery = useApiQuery<Item>(
      tenant && parsed.page === "spaces" && parsed.space
        ? base + "/spaces/" + parsed.space
        : null,
      { refetchInterval: live },
    ),
    appQuery = useApiQuery<Item>(
      tenant && parsed.page === "apps" && parsed.app
        ? base + "/apps/" + parsed.app
        : null,
      { refetchInterval: live },
    );
  // The sidebar's Spaces (the first page) and its Agents count (active,
  // unexpired grants: every grant in the Tenant for a security manager,
  // otherwise the person's own, as Home counts them).
  const canSpaces = !!capsQuery.data?.permissions["space.read"]?.available,
    manager = !!capsQuery.data?.permissions["authorization.manage"]?.all;
  const spacesQuery = useApiQuery<Page>(
      tenant && canSpaces ? base + "/spaces?limit=8" : null,
      { refetchInterval: live },
    ),
    grantsQuery = useApiQuery<Page<AgentGrant>>(
      tenant && capsQuery.data ? agentGrantsPath(tenant, manager) : null,
      { refetchInterval: live },
    );
  // Active, unexpired grants, measured when the grants were read.
  const activeAgents = grantsQuery.data
    ? grantsQuery.data.items.filter(
        (g) =>
          g.tenant_id === tenant &&
          g.state === "active" &&
          new Date(g.expires_at).getTime() > grantsQuery.dataUpdatedAt,
      ).length
    : undefined;
  return {
    overviewQuery,
    capsQuery,
    spaceQuery,
    appQuery,
    spacesQuery,
    canSpaces,
    manager,
    activeAgents,
  };
}
