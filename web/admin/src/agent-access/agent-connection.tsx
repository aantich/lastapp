import { Badge, Button, ConnectionCard, StatusBadge } from "../ui";
import type { AgentConfig, AgentGrant, Me } from "../api-types";
import { ago } from "../actors";
import { kindLabel, presetOf } from "./grant-values";

// One connection: the agent (its claimed name) acting for its owner inside
// a scope, the preset its permissions match, the permissions themselves,
// expiry and last use. `now` is when the grants were read.
export function AgentConnection({
  grant,
  config,
  me,
  tenant,
  now,
  revoking,
  edit,
  revoke,
}: {
  grant: AgentGrant;
  config?: AgentConfig;
  me: Me;
  tenant?: string;
  now: number;
  revoking: boolean;
  edit: () => void;
  revoke: () => void;
}) {
  const expired =
    grant.state === "active" && new Date(grant.expires_at).getTime() <= now;
  const active = grant.state === "active" && !expired;
  const preset = presetOf(config, grant.permissions_json);
  const scope =
    grant.scope_kind === "tenant"
      ? `Tenant ${grant.tenant_name || grant.scope_name}`
      : `${kindLabel(grant.scope_kind)} ${grant.scope_name}`;
  return (
    <ConnectionCard
      className="agent-connection"
      name={grant.client_name}
      subtitle={
        `for ${grant.owner_name} · ${scope}` +
        (!tenant && grant.tenant_name && grant.scope_kind !== "tenant"
          ? ` · ${grant.tenant_name}`
          : "")
      }
      status={
        expired ? (
          <StatusBadge state="expired" />
        ) : (
          <StatusBadge state={grant.state} />
        )
      }
      inactive={!active}
      chips={
        <>
          {config && (
            <Badge pill tone="info">
              {preset?.label || "Custom"}
            </Badge>
          )}
          {grant.permissions_json.map((code) => (
            <Badge
              key={code}
              pill
              mono
              title={config?.permissions.find((p) => p.code === code)?.label}
            >
              {code}
            </Badge>
          ))}
          {grant.permissions_json.includes("databases.provision") && (
            <Badge pill>
              At most {grant.limits_json.max_databases} new databases on{" "}
              {grant.limits_json.target_ids.length} approved targets
            </Badge>
          )}
        </>
      }
      meta={
        <>
          <span>
            {expired ? "Expired " : "Expires "}
            {ago(grant.expires_at)}
          </span>
          <span aria-hidden>·</span>
          <span>
            {grant.last_used_at
              ? `last used ${ago(grant.last_used_at)}`
              : "never used"}
          </span>
        </>
      }
      actions={
        active && (
          <>
            {grant.user_id === me.id && (
              <Button size="sm" variant="ghost" onClick={edit}>
                Edit
              </Button>
            )}
            <Button
              size="sm"
              variant="danger"
              loading={revoking}
              onClick={revoke}
            >
              Revoke
            </Button>
          </>
        )
      }
    />
  );
}
