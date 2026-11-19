import { Plus } from "lucide-react";
import { api } from "../api";
import { Button, CardHeader, Checkbox, Collection } from "../ui";
import { status } from "../console-forms";
import { num, type Item, type TenantCapabilities } from "../api-types";
import { personColumn } from "./person-column";

// The Tenant's members (People › Members): Tenant administrator role and
// Tenant access per person.
export function TenantMembers({
  base,
  caps,
  manageMembers,
  busy,
  act,
  toggle,
  addPerson,
}: {
  base: string;
  caps: TenantCapabilities;
  manageMembers: boolean;
  busy: boolean;
  act: (fn: () => Promise<unknown>) => Promise<void>;
  toggle: (r: Item, kind: "tenant") => Promise<void>;
  addPerson: () => void;
}) {
  return (
    <Collection
      path={base + "/memberships"}
      caption="Tenant members"
      empty="No one is a member of this Tenant yet."
      header={
        <CardHeader
          title="People"
          description="Members of this Tenant. A Tenant administrator assigns Tenant and team roles and Space/App managers; the role grants no access to business records."
          actions={
            manageMembers && (
              <Button
                variant="primary"
                icon={<Plus size={15} aria-hidden />}
                onClick={addPerson}
              >
                Add person
              </Button>
            )
          }
        />
      }
      columns={[
        personColumn,
        [
          "Tenant role",
          (r) => (
            <Checkbox
              label="Administrator"
              aria-label={"Tenant administrator for " + r.display_name}
              checked={!!r.administrator_id}
              disabled={
                busy ||
                !caps["authorization.manage"]?.all ||
                (!r.administrator_id &&
                  (r.state !== "active" ||
                    r.user_state !== "active" ||
                    !!r.platform_admin))
              }
              onChange={() => toggle(r, "tenant")}
            />
          ),
        ],
        ["Tenant access", (r) => status(r.state)],
        ["Account", (r) => status(r.user_state)],
      ]}
      actions={
        manageMembers
          ? (r) => (
              <Button
                size="sm"
                variant="ghost"
                disabled={busy}
                onClick={() =>
                  act(() =>
                    api(
                      base + "/memberships/" + r.user_id,
                      "PATCH",
                      {
                        state: r.state === "active" ? "disabled" : "active",
                      },
                      num(r.revision),
                    ),
                  )
                }
              >
                {r.state === "active"
                  ? "Disable Tenant access"
                  : "Enable Tenant access"}
              </Button>
            )
          : undefined
      }
    />
  );
}
