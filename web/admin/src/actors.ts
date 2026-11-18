import type { AvatarKind } from "./ui";
import { isItem, optText, text, type AgentGrant, type Item } from "./api-types";

// Who did an audited thing. The server classifies the actor once
// (internal/roles/audit.go: actor_kind is person, agent or system); an agent
// acts for the person named in actor_name and its grant is in the details
// (internal/op.AuditAs), named here from the grants the screen has read.
export function actorOf(
  e: Item,
  grants: AgentGrant[] = [],
): { name: string; kind: AvatarKind; label: string; via: string } {
  const details = isItem(e.details_json) ? e.details_json : {};
  const person = text(e.actor_name);
  if (e.actor_kind === "agent") {
    const grant = optText(details.agent_grant_id);
    const client =
      grants.find((g) => g.id === grant)?.client_name ||
      text(details.agent_client_id) ||
      "Agent";
    return {
      name: client,
      kind: "agent",
      label: `${client} for ${person}`,
      via: `Agent for ${person}`,
    };
  }
  if (e.actor_kind === "system")
    return { name: person, kind: "system", label: person, via: "Worker" };
  return { name: person, kind: "person", label: person, via: "Console" };
}

// "12 min ago", "3 h ago", "yesterday" — or "in 2 days" for a future time.
const relative = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
export function ago(at: unknown) {
  const ms = new Date(text(at)).getTime();
  if (!ms) return "";
  const s = (ms - Date.now()) / 1000;
  const [value, unit]: [number, Intl.RelativeTimeFormatUnit] =
    Math.abs(s) < 60
      ? [s, "second"]
      : Math.abs(s) < 3600
        ? [s / 60, "minute"]
        : Math.abs(s) < 86400
          ? [s / 3600, "hour"]
          : [s / 86400, "day"];
  return relative.format(Math.round(value), unit);
}

// The agent grants a Tenant screen may read to name agents: a Tenant
// security manager (authorization.manage) reads every grant in the Tenant,
// anyone else their own.
export const agentGrantsPath = (tenant: string, manager: boolean) =>
  manager
    ? `/v1/agent-access?limit=50&tenant_id=${encodeURIComponent(tenant)}`
    : "/v1/agent-access?limit=50";
