import { text, type Item, type TenantCapabilities } from "../api-types";

// What this account may do with a Space's or App's access, from its Tenant
// capabilities.
export function scopeAuthority(
  caps: TenantCapabilities,
  scopeKind: "space" | "app",
  scopeId: string,
  meta: Item,
) {
  // An App lists its own assignments, then its Space's, which apply to it by
  // inheritance, when this account may read the Space's (the server checks
  // again). Only the App's own are changed here: App access never widens
  // what the Space grants.
  const readsSpace =
    scopeKind === "app" &&
    !!meta.space_id &&
    (!!caps["authorization.read"]?.all ||
      (caps["authorization.read"]?.spaces || []).includes(text(meta.space_id)));
  const canManage =
    !!caps["authorization.manage"]?.all ||
    (caps["authorization.manage"]?.spaces || []).includes(
      scopeKind === "space" ? scopeId : text(meta.space_id),
    ) ||
    (caps["authorization.manage"]?.apps || []).includes(scopeId);
  const canDelegate =
    !!caps["authorization.manage"]?.all ||
    (scopeKind === "app" &&
      (caps["authorization.manage"]?.spaces || []).includes(
        text(meta.space_id),
      ));
  const canEditEntities =
    !!caps["entity.manage"]?.all ||
    (caps["entity.manage"]?.spaces || []).includes(scopeId);
  return { readsSpace, canManage, canDelegate, canEditEntities };
}
