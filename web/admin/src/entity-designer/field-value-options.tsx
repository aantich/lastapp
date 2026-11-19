import {
  Checkbox,
  Columns,
  Disclosure,
  FormField,
  Select,
  Stack,
  TextField,
} from "../ui";
import { text, type EntityCatalog, type Item } from "../api-types";
import { Expression } from "./expression";
import { inputValue, isExpression, optionalText } from "./shared";

// Where a field's value comes from, how it can be queried, and its key.
export function FieldValueOptions({
  f,
  field,
  fields,
  catalog,
  patch,
}: {
  f: Item;
  field: Item;
  fields: Item[];
  catalog: EntityCatalog;
  patch: (v: Item) => void;
}) {
  const expression = f.calculation || f.default;
  return (
    <Disclosure summary="Defaults, calculation and query options">
      <Stack gap="md">
        <FormField label="Value source">
          <Select
            value={
              f.calculation ? "calculation" : f.default ? "default" : "input"
            }
            onChange={(source) =>
              patch({
                default: source === "default" ? { literal: "" } : undefined,
                calculation:
                  source === "calculation"
                    ? { field: fields[0]?.key || "" }
                    : undefined,
              })
            }
            options={[
              { value: "input", label: "User input" },
              { value: "default", label: "Default when omitted" },
              { value: "calculation", label: "Calculated" },
            ]}
          />
        </FormField>
        {isExpression(expression) && (
          <Expression
            value={expression}
            onChange={(value) =>
              patch(f.calculation ? { calculation: value } : { default: value })
            }
            fields={fields}
            catalog={catalog}
            label={f.calculation ? "Formula" : "Default"}
            formula={!!f.calculation}
            defaultOnly={!!f.default}
            contextType={f.default ? optionalText(f.type) : undefined}
            allowContext={!f.calculation}
          />
        )}
        <Columns min={160} gap="sm">
          {["filterable", "sortable"].map((key) => (
            <Checkbox
              key={key}
              label={key === "filterable" ? "Allow filtering" : "Allow sorting"}
              checked={!!f[key]}
              onChange={(on) => patch({ [key]: on })}
            />
          ))}
        </Columns>
        <Columns min={220} gap="md">
          <FormField label="Uniqueness">
            <Select
              value={text(f.unique)}
              onChange={(unique) => patch({ unique })}
              options={[
                { value: "", label: "None" },
                { value: "entity", label: "Across this entity" },
                { value: "owner", label: "Per owner" },
                { value: "team", label: "Per team" },
              ]}
            />
          </FormField>
          <TextField
            label="Stable key"
            hint={
              field.key
                ? "Apps and stored data use this key; it cannot change."
                : undefined
            }
            required
            pattern="[a-z][a-z0-9_]*"
            value={inputValue(f.key)}
            disabled={!!field.key}
            onChange={(e) => patch({ key: e.target.value })}
          />
        </Columns>
      </Stack>
    </Disclosure>
  );
}
