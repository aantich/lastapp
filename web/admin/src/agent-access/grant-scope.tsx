import { Columns, FlowSection, FormField, RemoteSelect, Select } from "../ui";
import type { AgentPermission, Me } from "../api-types";
import { kindLabel, type GrantValuesProps } from "./grant-values";

// Where the agent may work: the Tenant and the scope inside it.
export function GrantScope({
  me,
  values,
  setValues,
  eligiblePermissions,
}: GrantValuesProps & {
  me: Me;
  eligiblePermissions: (kind: string) => AgentPermission[];
}) {
  const scopePath =
    values.scope_kind === "space"
      ? "spaces"
      : values.scope_kind === "app"
        ? "apps"
        : "teams";
  return (
    <FlowSection label="Where may it work?">
      <Columns min={200} gap="md">
        {!me.platform_admin && (
          <FormField label="Tenant" required>
            <RemoteSelect
              path="/v1/my-tenants"
              label="Tenant"
              value={values.tenant_id}
              required
              initial={(me.tenants || []).map((t) => ({
                value: t.id,
                label: t.display_name,
              }))}
              onChange={(id) =>
                setValues({ ...values, tenant_id: id, scope_id: "" })
              }
            />
          </FormField>
        )}
        <FormField label="Access scope">
          <Select
            value={values.scope_kind}
            onChange={(scope_kind) =>
              setValues({
                ...values,
                scope_kind,
                scope_id: "",
                permissions: values.permissions.filter((code) =>
                  eligiblePermissions(scope_kind).some((p) => p.code === code),
                ),
              })
            }
            options={(me.platform_admin
              ? ["platform"]
              : ["tenant", "space", "app", "team"]
            ).map((kind) => ({
              value: kind,
              label:
                kind === "tenant"
                  ? "Entire Tenant, including future resources"
                  : kindLabel(kind),
            }))}
          />
        </FormField>
        {!["tenant", "platform"].includes(values.scope_kind) &&
          values.tenant_id && (
            <FormField label={`Choose ${values.scope_kind}`} required>
              <RemoteSelect
                key={scopePath + values.tenant_id}
                path={`/v1/tenants/${values.tenant_id}/${scopePath}`}
                label={`Choose ${values.scope_kind}`}
                required
                value={values.scope_id}
                initial={[]}
                onChange={(id) => setValues({ ...values, scope_id: id })}
              />
            </FormField>
          )}
      </Columns>
    </FlowSection>
  );
}
