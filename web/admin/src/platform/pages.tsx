import { api } from "../api";
import { num, text } from "../api-types";
import { Button, Checkbox, Collection, confirmAction } from "../ui";
import { useBuiltinRoles } from "../console-config";
import { addToTenant } from "./forms";
import { columnsName, columnsStatus, type PlatformScreen } from "./screen";

export { PlatformSettings } from "./settings-page";
export { PlatformTenant } from "./tenant-page";

// Every Tenant on the platform.
export function TenantsPage({ p }: { p: PlatformScreen }) {
  const { go } = p;
  return (
    <Collection
      path="/admin/v1/tenants"
      caption="Tenants"
      columns={[
        columnsName,
        ["Plan", (r) => text(r.plan_name)],
        ["Spaces", (r) => text(r.space_count)],
        ["Apps", (r) => text(r.app_count)],
        columnsStatus,
      ]}
      actions={(r) => (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => go("tenants", text(r.id), "overview")}
        >
          Open
        </Button>
      )}
    />
  );
}
// Every account, with platform administrator and sign-in controls.
export function UsersPage({ p }: { p: PlatformScreen }) {
  const { act, busy } = p;
  const roles = useBuiltinRoles();
  return (
    <Collection
      path="/admin/v1/users"
      caption="Accounts"
      columns={[
        columnsName,
        [
          "Sign-in",
          (r) =>
            r.oidc_issuer === "urn:lastapp:local"
              ? "Email/password"
              : "Single sign-on",
        ],
        columnsStatus,
        [
          "Platform admin",
          (r) => (
            <Checkbox
              aria-label={"Platform administrator for " + r.display_name}
              checked={!!r.platform_admin}
              disabled={
                busy || (!r.platform_assignment_id && r.state !== "active")
              }
              onChange={() =>
                act(() =>
                  r.platform_assignment_id
                    ? api(
                        "/admin/v1/role-assignments/" +
                          r.platform_assignment_id,
                        "DELETE",
                        undefined,
                        num(r.platform_assignment_revision),
                      )
                    : api("/admin/v1/role-assignments", "POST", {
                        id: crypto.randomUUID(),
                        user_id: r.id,
                        role_id: roles.platform,
                      }),
                )
              }
            />
          ),
        ],
      ]}
      actions={(r) => (
        <>
          {!r.platform_admin && r.state === "active" && (
            <Button size="sm" variant="ghost" onClick={() => addToTenant(p, r)}>
              Add to Tenant
            </Button>
          )}
          <Button
            size="sm"
            variant={r.state === "active" ? "danger" : "secondary"}
            disabled={busy}
            onClick={async () => {
              const disable = r.state === "active";
              if (
                disable &&
                !(await confirmAction({
                  title: "Disable account?",
                  message: `${text(r.display_name)} can no longer sign in and their sessions end. You can enable the account again later.`,
                  confirmLabel: "Disable account",
                  danger: true,
                }))
              )
                return;
              act(() =>
                api(
                  "/admin/v1/users/" + r.id,
                  "PATCH",
                  { state: disable ? "disabled" : "active" },
                  num(r.revision),
                ),
              );
            }}
          >
            {r.state === "active" ? "Disable account" : "Enable account"}
          </Button>
        </>
      )}
    />
  );
}
