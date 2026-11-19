import { useState } from "react";
import { Copy } from "lucide-react";
import {
  Alert,
  Button,
  CodeBlock,
  Dialog,
  DialogBody,
  Inline,
  Skeleton,
  Stack,
  Icon,
} from "./ui";
import { useApiQuery } from "./query";
import type { AgentConfig } from "./api-types";

export const agentConfigPath = "/v1/agent-access/config";

// How an agent connects: the server's MCP URL (GET /v1/agent-access/config)
// with a copy button. A connection always starts in the agent client, which
// opens Lastapp sign-in and consent (MCP.md); the console has no operation
// that starts one. Shared by the Connect agent dialog and Agent access.
export function McpConnection({ config }: { config: AgentConfig }) {
  const [copied, setCopied] = useState(false),
    [error, setError] = useState("");
  return (
    <Stack gap="sm">
      <Inline gap="md">
        <strong>Connect from your MCP client</strong>
        <Button
          size="sm"
          icon={<Icon icon={Copy} size="sm" aria-hidden />}
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(config.mcp_url);
              setCopied(true);
            } catch {
              setError("Select and copy the MCP URL below.");
            }
          }}
        >
          {copied ? "URL copied" : "Copy MCP URL"}
        </Button>
      </Inline>
      <CodeBlock label="MCP URL">{config.mcp_url}</CodeBlock>
      <small>
        Add this URL in your agent client. It opens Lastapp sign-in and asks you
        to approve access. No API keys or OAuth client setup required.
      </small>
      {error && <Alert>{error}</Alert>}
    </Stack>
  );
}

// The top bar's "Connect agent": what to give the agent client and what
// happens next, with the way to review connections afterwards.
export function ConnectAgentDialog({
  tenantName,
  agentAccess,
  close,
}: {
  tenantName: string;
  agentAccess: () => void;
  close: () => void;
}) {
  const config = useApiQuery<AgentConfig>(agentConfigPath);
  return (
    <Dialog
      title="Connect an agent"
      description={`Let an MCP client work in ${tenantName || "this Tenant"} with access you approve.`}
      onClose={close}
      footer={
        <>
          <Button onClick={close}>Close</Button>
          <Button variant="primary" onClick={agentAccess}>
            Review agent access
          </Button>
        </>
      }
    >
      <DialogBody>
        <Stack>
          {config.data ? (
            <McpConnection config={config.data} />
          ) : config.error ? (
            <Alert>{config.error.message}</Alert>
          ) : (
            <Skeleton width="70%" />
          )}
          <ol className="ui-steps">
            <li>Add the MCP URL in your agent client and connect.</li>
            <li>
              Sign in to Lastapp when the client opens it, then choose one
              Tenant, team, Space or App, the actions the agent may take and
              when its access expires.
            </li>
            <li>
              Back in the client, the agent works within that approval. Review
              or revoke it any time under Agents.
            </li>
          </ol>
          <small>
            An agent never gets business records, database credentials or SQL.
          </small>
        </Stack>
      </DialogBody>
    </Dialog>
  );
}
