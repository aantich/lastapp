import { consoleURL, useConsoleURL } from "../console-url";
import { ContextLabel, ContextSwitcher } from "../ui";
import { savePreference } from "../preferences";
import type { Me } from "../api-types";
import { options } from "./screen";

// The sidebar's Tenant context: the shell's `context` (the switcher) and
// `section` (the label shown without one).
export function tenantContext({
  me,
  memberships,
  tenant,
  tenantName,
  navigate,
}: {
  me: Me;
  memberships: Me["tenants"];
  tenant: string;
  tenantName: string;
  navigate: ReturnType<typeof useConsoleURL>[1];
}) {
  function selectTenant(id: string) {
    if (!id) return;
    savePreference("lastapp-tenant", id);
    navigate(consoleURL.tenant(id, "home"));
  }
  // The Tenant switcher is always shown when the account has a Tenant, even
  // just one, so switching is discoverable: a control under the brand row
  // that pages /v1/my-tenants. Without one, the section label over the
  // screens says so.
  const switchable = !!memberships.length || !!me.pagination?.has_more;
  const tenantSwitcher = switchable
    ? (rail: boolean) => (
        <ContextSwitcher
          kind="Tenant"
          variant="sidebar"
          rail={rail}
          className="company-name"
          path="/v1/my-tenants"
          value={tenant}
          current={tenantName}
          options={options(memberships)}
          onChange={selectTenant}
        />
      )
    : undefined;
  const tenantSection = switchable ? undefined : (
    <ContextLabel
      className="company-name"
      value={tenantName || "No Tenant access"}
    />
  );
  return { context: tenantSwitcher, section: tenantSection };
}
