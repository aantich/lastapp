import { useState } from "react";
import { Checkbox, Columns, FormField, Select, TextField } from "../ui";
import { isItem, type EntityCatalog, type Item } from "../api-types";
import { EditorForm } from "./editor-dialog";
import { FieldTypeOptions } from "./field-type-options";
import { FieldValueOptions } from "./field-value-options";
import { fieldLabel, inputValue, keyOf, optionalText, strings } from "./shared";

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
      <FieldTypeOptions
        f={f}
        patch={patch}
        catalog={catalog}
        space={space}
        choice={choice}
        setChoice={setChoice}
        choices={choices}
      />
      <FieldValueOptions
        f={f}
        field={field}
        fields={fields}
        catalog={catalog}
        patch={patch}
      />
    </EditorForm>
  );
}
