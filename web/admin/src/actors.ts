import type { AvatarKind } from "./ui";
import {
  isItem,
  optText,
  text,
  type AgentConfig,
  type AgentGrant,
  type Item,
} from "./api-types";

// Who did an audited thing. The server classifies the actor once
// (internal/roles/audit.go: actor_kind is person, agent or system); an agent
// acts for the person named in actor_name and its grant is in the details
// (internal/op.AuditAs), named here from the grants the screen has read.
// `via` says how: "for <person>" (with the grant's preset when its
// permissions are exactly one of `presets`, GET /v1/agent-access/config),
// "worker" or "console".
export function actorOf(
  e: Item,
  grants: AgentGrant[] = [],
  presets: AgentPreset[] = [],
): { name: string; kind: AvatarKind; label: string; via: string } {
  const details = isItem(e.details_json) ? e.details_json : {};
  const person = text(e.actor_name);
  if (e.actor_kind === "agent") {
    const id = optText(details.agent_grant_id);
    const grant = grants.find((g) => g.id === id);
    const client =
      grant?.client_name || text(details.agent_client_id) || "Agent";
    const preset = grant && presetOf(grant.permissions_json, presets);
    return {
      name: client,
      kind: "agent",
      label: `${client} for ${person}`,
      via: `for ${person}` + (preset ? ` · ${preset}` : ""),
    };
  }
  if (e.actor_kind === "system")
    return { name: person, kind: "system", label: person, via: "worker" };
  return { name: person, kind: "person", label: person, via: "console" };
}

// The Activity filter's actor choices, the server's actor kinds
// (roles.AuditFilter.Actor; the worker is the system actor). The command
// palette offers the same filters.
export const auditActors = [
  { value: "", label: "All" },
  { value: "person", label: "People" },
  { value: "agent", label: "Agents" },
  { value: "system", label: "Worker" },
];

// The Activity page preset to one actor kind (…/activity?actor=agent).
export const activityQuery = (actor: string) =>
  actor ? "?actor=" + encodeURIComponent(actor) : "";

export type AgentPreset = AgentConfig["presets"][number];

// The preset whose permissions are exactly these (in any order), if any.
export function presetOf(permissions: string[] = [], presets: AgentPreset[]) {
  const key = (codes: string[]) => [...codes].sort().join(" ");
  return presets.find((p) => key(p.permissions) === key(permissions))?.label;
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
