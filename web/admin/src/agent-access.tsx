import { useState } from "react";
import { Bot, RefreshCw } from "lucide-react";
import { api } from "./api";
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardHeader,
  ConnectionGrid,
  confirmAction,
  EmptyState,
  Inline,
  Skeleton,
  Stack,
} from "./ui";
import { useAction } from "./use-action";
import { agentConfigPath, McpConnection } from "./connect-agent";
import { refreshQueries, useApiQuery } from "./query";
import type { AgentConfig, AgentGrant, Me, Page } from "./api-types";
import { AgentGrantDialog } from "./agent-access/grant-dialog";
import { AgentConnection } from "./agent-access/agent-connection";

export { AgentConsent } from "./agent-access/agent-consent";

// Agent connections: a Tenant's (for a Tenant security manager, `tenant`)
// or the person's own. With `heading` the list sits under an "Agent access"
// card with the MCP URL (account and Tenant settings); the Agents page
// draws its own header instead.
export function AgentAccess({
  me,
  tenant,
  heading = true,
}: {
  me: Me;
  tenant?: string;
  heading?: boolean;
}) {
  const [page, setPage] = useState(1),
    [actionError, setError] = useState(""),
    [editing, setEditing] = useState<AgentGrant | null>(null),
    [revoking, setRevoking] = useState("");
  const configQuery = useApiQuery<AgentConfig>(agentConfigPath),
    listQuery = useApiQuery<Page<AgentGrant>>(
      `/v1/agent-access?page=${page}&limit=20${tenant ? "&tenant_id=" + tenant : ""}`,
    );
  const revoke = useAction(page, { onSuccess: () => {}, onError: setError });
  const config = configQuery.data,
    loading = configQuery.isPending || listQuery.isPending,
    loadFailed = !!(configQuery.error || listQuery.error),
    items = listQuery.data?.items || [],
    more = !!listQuery.data?.pagination?.has_more,
    error =
      actionError ||
      configQuery.error?.message ||
      listQuery.error?.message ||
      "";
  function load() {
    setError("");
    void configQuery.refetch();
    void listQuery.refetch();
  }
  async function confirmRevoke(grant: AgentGrant) {
    if (
      !(await confirmAction({
        title: "Revoke agent access?",
        message: `${grant.client_name} stops acting for ${grant.owner_name} in ${grant.scope_name} at once. A revoked connection cannot be restored; the agent must connect again.`,
        confirmLabel: "Revoke access",
        danger: true,
      }))
    )
      return;
    setRevoking(grant.id);
    void revoke.run(() =>
      api(
        `/v1/agent-access/${grant.id}`,
        "PATCH",
        { state: "revoked" },
        grant.revision,
      ),
    );
  }
  return (
    <Stack gap="md" className="agent-access">
      {heading && (
        <Card>
          <CardHeader
            title="Agent access"
            description={
              tenant
                ? "Review connections approved by members of this Tenant and revoke access when needed."
                : "Review the clients acting on your behalf and revoke them when finished."
            }
            actions={
              <Button
                icon={<RefreshCw size={15} aria-hidden />}
                disabled={configQuery.isFetching || listQuery.isFetching}
                onClick={load}
              >
                Refresh connections
              </Button>
            }
          />
          {config && (
            <CardBody>
              <McpConnection config={config} />
            </CardBody>
          )}
        </Card>
      )}
      {error && <Alert>{error}</Alert>}
      {loading ? (
        <Card>
          <CardBody>
            <Stack gap="sm">
              <Skeleton width="40%" />
              <Skeleton width="65%" />
            </Stack>
          </CardBody>
        </Card>
      ) : loadFailed ? null : items.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Bot size={28} aria-hidden />}
            title="No agent connections yet"
          >
            Add the MCP endpoint to your agent client to begin.
          </EmptyState>
        </Card>
      ) : (
        <ConnectionGrid>
          {items.map((grant) => (
            <AgentConnection
              key={grant.id}
              grant={grant}
              config={config}
              me={me}
              tenant={tenant}
              now={listQuery.dataUpdatedAt}
              revoking={revoke.busy && revoking === grant.id}
              edit={() => setEditing(grant)}
              revoke={() => void confirmRevoke(grant)}
            />
          ))}
        </ConnectionGrid>
      )}
      {(page > 1 || more) && (
        <Inline>
          <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
            Previous
          </Button>
          <span>Page {page}</span>
          <Button disabled={!more} onClick={() => setPage(page + 1)}>
            Next
          </Button>
        </Inline>
      )}
      {editing && config && (
        <AgentGrantDialog
          key={editing.id}
          grant={editing}
          config={config}
          me={me}
          close={() => setEditing(null)}
          done={() => {
            setEditing(null);
            void refreshQueries();
          }}
        />
      )}
    </Stack>
  );
}
