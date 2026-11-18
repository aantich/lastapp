import { useState } from "react";
import {
  Checkbox,
  Columns,
  Disclosure,
  FormField,
  NumberInput,
  RemoteSelect,
  Select,
  Stack,
  TextField,
} from "../ui";
import { isItem, text, type EntityCatalog, type Item } from "../api-types";
import { Expression } from "./expression";
import { EditorForm } from "./editor-dialog";
import {
  choiceLabel,
  fieldLabel,
  inputValue,
  isExpression,
  keyOf,
  optionalText,
  strings,
} from "./shared";

// Adds or edits one field of the draft definition.
export function FieldEditor({
  field,
  fields,
  catalog,
  space,
  published,
  save,
}: {
  field: Item;
  fields: Item[];
  catalog: EntityCatalog;
  space: string;
  published: boolean;
  save: (v: Item) => void;
}) {
  const [f, set] = useState<Item>({ ...field }),
    [error, setError] = useState(""),
    [choice, setChoice] = useState<string>(strings(field.values).join(", "));
  const patch = (v: Item) => set({ ...f, ...v });
  const expression = f.calculation || f.default;
  const choices = choice
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);
  return (
    <EditorForm
      submitLabel="Save field"
      error={error}
      submit={() => {
        if (fields.some((x) => x.key === f.key && x.key !== field.key)) {
          setError(
            "Another field already uses this key. Choose a different key.",
          );
          return;
        }
        const value = { ...f };
        if (value.type === "choice") value.values = choices;
        if (value.type === "choice" && isItem(value.value_labels)) {
          const values = strings(value.values);
          value.value_labels = Object.fromEntries(
            Object.entries(value.value_labels).filter(([key]) =>
              values.includes(key),
            ),
          );
        }
        save(value);
      }}
    >
      <Columns min={220} gap="md">
        <TextField
          label="Name"
          required
          data-autofocus
          value={inputValue(f.label)}
          onChange={(e) =>
            patch({
              label: e.target.value,
              ...(!field.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
        <FormField
          label="Type"
          hint={published ? "A published field keeps its type." : undefined}
        >
          <Select
            value={optionalText(f.type)}
            disabled={published}
            onChange={(type) =>
              set({
                ...catalog.field_defaults[type],
                key: f.key,
                label: f.label,
              })
            }
            options={catalog.types.map((type: string) => ({
              value: type,
              label: fieldLabel(type),
            }))}
          />
        </FormField>
      </Columns>
      <Checkbox
        label="Required"
        checked={!!f.required}
        onChange={(required) => patch({ required })}
      />
      {f.type === "text" && (
        <FormField label="Maximum length" required>
          <NumberInput
            required
            max={catalog.field_constraints.max_text_length}
            min={catalog.field_constraints.min_text_length}
            value={inputValue(f.max_length)}
            onChange={(n) => patch({ max_length: Number(n) })}
          />
        </FormField>
      )}
      {f.type === "decimal" && (
        <Columns min={160} gap="md">
          {["precision", "scale"].map((key) => (
            <FormField label={fieldLabel(key)} required key={key}>
              <NumberInput
                required
                value={inputValue(f[key])}
                onChange={(n) => patch({ [key]: Number(n) })}
              />
            </FormField>
          ))}
        </Columns>
      )}
      {f.type === "choice" && (
        <TextField
          label="Choices, separated by commas"
          required
          value={choice}
          onChange={(e) => setChoice(e.target.value)}
        />
      )}
      {f.type === "choice" && choices.length > 0 && (
        <Disclosure summary="Choice display labels">
          <Stack gap="md">
            <p>Keep keys stable; change their labels here.</p>
            <Columns min={200} gap="md">
              {choices.map((key) => (
                <TextField
                  key={key}
                  label={key}
                  value={choiceLabel(f, key)}
                  onChange={(e) =>
                    patch({
                      value_labels: {
                        ...(isItem(f.value_labels) ? f.value_labels : {}),
                        [key]: e.target.value,
                      },
                    })
                  }
                />
              ))}
            </Columns>
          </Stack>
        </Disclosure>
      )}
      {f.type === "reference" && (
        <FormField label="Related entity">
          <RemoteSelect
            path={"/v1/spaces/" + space + "/entities"}
            label="Related entity"
            value={text(f.target)}
            initial={[]}
            labelKey="label"
            onChange={(value) => patch({ target: value })}
          />
        </FormField>
      )}
      {["integer", "decimal"].includes(text(f.type)) && (
        <Columns min={160} gap="md">
          {["minimum", "maximum"].map((key) => (
            <TextField
              key={key}
              label={fieldLabel(key)}
              inputMode={f.type === "integer" ? "numeric" : "decimal"}
              value={inputValue(f[key]) ?? ""}
              onChange={(e) =>
                patch({
                  [key]:
                    e.target.value === ""
                      ? undefined
                      : f.type === "integer"
                        ? Number(e.target.value)
                        : e.target.value,
                })
              }
            />
          ))}
        </Columns>
      )}
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
                patch(
                  f.calculation ? { calculation: value } : { default: value },
                )
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
                label={
                  key === "filterable" ? "Allow filtering" : "Allow sorting"
                }
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
    </EditorForm>
  );
}
