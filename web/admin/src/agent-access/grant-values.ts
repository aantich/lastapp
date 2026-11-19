import type { AgentConfig, AgentGrant, AgentLimits, Me } from "../api-types";

export type GrantValues = {
  tenant_id: string;
  scope_kind: string;
  scope_id: string;
  permissions: string[];
  limits: AgentLimits;
  days: number;
};
// What each step of the grant form reads and changes.
export type GrantValuesProps = {
  values: GrantValues;
  setValues: (values: GrantValues) => void;
};
export function initialGrant(
  config: AgentConfig,
  me: Me,
  grant?: AgentGrant,
): GrantValues {
  return {
    tenant_id:
      grant?.tenant_id || (me.platform_admin ? "" : me.tenants?.[0]?.id || ""),
    scope_kind:
      grant?.scope_kind || (me.platform_admin ? "platform" : "tenant"),
    scope_id: grant?.scope_id || "",
    permissions: grant?.permissions_json || config.default_permissions,
    limits: grant?.limits_json || config.default_limits,
    days: config.default_days,
  };
}
const sameSet = (a: string[], b: string[]) =>
  a.length === b.length && a.every((code) => b.includes(code));
export const dayLabel = (d: number) => `${d} ${d === 1 ? "day" : "days"}`;
export const kindLabel = (kind: string) =>
  kind ? kind[0].toUpperCase() + kind.slice(1) : "";

// The preset whose permissions are exactly these, if any. A grant stores
// permissions, never a preset, so this is a reading of them.
export const presetOf = (config: AgentConfig | undefined, codes: string[]) =>
  config?.presets.find((p) => sameSet(p.permissions, codes));
