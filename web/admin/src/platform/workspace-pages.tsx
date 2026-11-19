import { AccountPage } from "../account-page";
import { ActivityLog } from "../activity-log";
import type { PlatformScreen } from "./screen";
import { TenantsPage, UsersPage } from "./pages";
import { PlatformSettings } from "./settings-page";
import { PlatformTenant } from "./tenant-page";

// The page the Platform route names: the open Tenant, a top-level list,
// the audit log, settings or the operator's own account.
export function PlatformPages({
  p,
  refreshMe,
}: {
  p: PlatformScreen;
  refreshMe: () => Promise<void>;
}) {
  const { route, tenant } = p,
    { page } = route;
  return tenant ? (
    <PlatformTenant p={p} />
  ) : page === "tenants" ? (
    <TenantsPage p={p} />
  ) : page === "users" ? (
    <UsersPage p={p} />
  ) : page === "activity" ? (
    <ActivityLog
      path={
        route.id
          ? "/v1/tenants/" + route.id + "/audit-events"
          : "/admin/v1/audit-events"
      }
    />
  ) : page === "settings" ? (
    <PlatformSettings p={p} />
  ) : page === "account" ? (
    <AccountPage me={p.me} refreshMe={refreshMe} />
  ) : null;
}
