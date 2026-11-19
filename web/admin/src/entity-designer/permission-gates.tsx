import { Plus } from "lucide-react";
import { useListKeys } from "../list-keys";
import { items, text, type EntityCatalog, type Item } from "../api-types";
import {
  Button,
  Checkbox,
  FieldGroup,
  FormField,
  Inline,
  Select,
  Stack,
  Icon,
} from "../ui";
import { Expression } from "./expression";
import { FieldChoices } from "./field-choices";
import { isExpression, optionalText, strings } from "./shared";

// One list of an App's permission gates (its ceiling or its restrictions).
// A gate picks a Space permission path and may narrow its fields, records
// and App roles.
export function PermissionGates({
  access,
  edit,
  name: key,
  keys,
  permissions,
  roles,
  catalog,
}: {
  access: Item;
  edit: (v: Item) => void;
  name: "ceiling" | "restrictions";
  keys: ReturnType<typeof useListKeys>;
  permissions: (Item & { entity: Item })[];
  roles: Item[];
  catalog: EntityCatalog | null;
}) {
  const list = items(access[key]);
  const change = (i: number, v: Item) =>
    edit({ ...access, [key]: list.map((x, j) => (i === j ? v : x)) });
  return (
    <Stack gap="md">
      {list.map((g, i) => {
        const p = permissions.find((p) => p.id === g.permission);
        const fields = items(p?.entity.fields);
        return (
          <FieldGroup
            key={keys.keys[i]}
            legend={"Permission gate " + (i + 1)}
            framed
            actions={
              <Button
                variant="danger"
                size="sm"
                onClick={() => {
                  keys.remove(i);
                  edit({
                    ...access,
                    [key]: list.filter((_, j) => j !== i),
                  });
                }}
              >
                Remove gate
              </Button>
            }
          >
            <FormField label="Base Space permission">
              <Select
                value={optionalText(g.permission)}
                onChange={(permission) => change(i, { permission, fields: [] })}
                options={[
                  { value: "", label: "Choose permission" },
                  ...permissions.map((p) => ({
                    value: text(p.id),
                    label: `${p.entity.label} · ${p.label} (${p.operation})`,
                  })),
                ]}
              />
            </FormField>
            {p && (
              <FieldChoices
                legend="Fields"
                description="Only fields the Space permission already reaches."
                fields={fields.filter(
                  (f) => Array.isArray(p.fields) && p.fields.includes(f.key),
                )}
                value={strings(g.fields)}
                onChange={(v) => change(i, { ...g, fields: v })}
                empty="This permission reaches the record, not fields."
              />
            )}
            <Checkbox
              label="Limit matching records"
              checked={!!g.when}
              onChange={(on) =>
                change(i, {
                  ...g,
                  when: on
                    ? {
                        op: "is_not_null",
                        args: [{ field: fields[0]?.key || "" }],
                      }
                    : undefined,
                })
              }
            />
            {isExpression(g.when) && p && catalog && (
              <Expression
                fields={fields}
                catalog={catalog}
                value={g.when}
                onChange={(v) => change(i, { ...g, when: v })}
              />
            )}
            <FieldChoices
              legend="Optional App roles"
              fields={roles.map((r) => ({
                key: r.id,
                label: r.display_name,
              }))}
              value={strings(g.roles)}
              onChange={(v) => change(i, { ...g, roles: v })}
              empty="This App has no roles yet."
            />
          </FieldGroup>
        );
      })}
      <Inline>
        <Button
          size="sm"
          icon={<Icon icon={Plus} size="sm" aria-hidden />}
          onClick={() =>
            edit({
              ...access,
              [key]: [...list, { permission: "", fields: [] }],
            })
          }
        >
          Add permission gate
        </Button>
      </Inline>
    </Stack>
  );
}
