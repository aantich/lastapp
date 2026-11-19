import {
  Badge,
  Checkbox,
  ChoiceCards,
  Disclosure,
  FlowSection,
  Inline,
  Stack,
} from "../ui";
import type { AgentConfig, AgentPermission } from "../api-types";
import { presetOf, type GrantValuesProps } from "./grant-values";

// What the agent may do: a preset, or individual permissions from
// `permissionList` (those grantable here).
export function GrantPermissions({
  config,
  values,
  setValues,
  permissionList,
}: GrantValuesProps & {
  config: AgentConfig;
  permissionList: AgentPermission[];
}) {
  // A preset is offered only when every permission it names may be granted
  // here (the client requested it and the scope kind allows it).
  const grantable = (codes: string[]) =>
    codes.every((code) => permissionList.some((p) => p.code === code));
  const preset = presetOf(config, values.permissions);
  function permissions(codes: string[]) {
    setValues({
      ...values,
      permissions: codes.filter((code) =>
        permissionList.some((p) => p.code === code),
      ),
    });
  }
  return (
    <FlowSection label="What may it do?">
      <ChoiceCards
        label="Permission presets"
        options={config.presets.map((p) => ({
          value: p.label,
          label: p.label,
          description: p.description,
          disabled: !grantable(p.permissions),
        }))}
        value={preset?.label || ""}
        onChange={(label) =>
          permissions(
            config.presets.find((p) => p.label === label)?.permissions || [],
          )
        }
      />
      <Inline gap="sm">
        {values.permissions.map((code) => (
          <Badge key={code} pill mono>
            {code}
          </Badge>
        ))}
      </Inline>
      <Disclosure
        summary="Choose individual permissions"
        meta={
          preset
            ? "Presets only tick boxes"
            : `${values.permissions.length} checked`
        }
        defaultOpen={!preset}
      >
        <Stack gap="sm">
          {permissionList.map((permission) => (
            <Checkbox
              key={permission.code}
              label={permission.label}
              description={permission.description}
              checked={values.permissions.includes(permission.code)}
              onChange={(checked) =>
                permissions(
                  checked
                    ? [...values.permissions, permission.code]
                    : values.permissions.filter((p) => p !== permission.code),
                )
              }
            />
          ))}
        </Stack>
      </Disclosure>
    </FlowSection>
  );
}
