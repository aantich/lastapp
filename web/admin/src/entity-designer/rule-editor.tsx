import { useState } from "react";
import { Checkbox, Columns, FormField, Select, TextField } from "../ui";
import { text, type EntityCatalog, type Item } from "../api-types";
import { Expression } from "./expression";
import { EditorForm } from "./editor-dialog";
import { inputValue, isExpression, keyOf, optionalText } from "./shared";

// Adds or edits one validation or allowed-change rule.
export function RuleEditor({
  value,
  fields,
  catalog,
  save,
}: {
  value: Item;
  fields: Item[];
  catalog: EntityCatalog;
  save: (v: Item) => void;
}) {
  const [r, set] = useState({ ...value });
  return (
    <EditorForm submitLabel="Save rule" submit={() => save(r)}>
      <Columns min={220} gap="md">
        <TextField
          label="Name"
          required
          data-autofocus
          value={text(r.name)}
          onChange={(e) =>
            set({
              ...r,
              name: e.target.value,
              ...(!value.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
        <FormField label="Rule type">
          <Select
            value={optionalText(r.kind)}
            onChange={(kind) => set({ ...r, kind })}
            options={[
              { value: "validation", label: "Validate resulting record" },
              { value: "change", label: "Allowed change" },
            ]}
          />
        </FormField>
      </Columns>
      <Checkbox
        label="Apply only when"
        checked={!!r.when}
        onChange={(on) =>
          set({
            ...r,
            when: on
              ? {
                  op: "eq",
                  args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                }
              : undefined,
          })
        }
      />
      {isExpression(r.when) && (
        <Expression
          value={r.when}
          onChange={(v) => set({ ...r, when: v })}
          fields={fields}
          catalog={catalog}
          transition={r.kind === "change"}
          label="When"
          allowContext={false}
        />
      )}
      {isExpression(r.assert) && (
        <Expression
          value={r.assert}
          onChange={(v) => set({ ...r, assert: v })}
          fields={fields}
          catalog={catalog}
          transition={r.kind === "change"}
          label="Must be true"
          allowContext={false}
        />
      )}
      <Columns min={220} gap="md">
        <TextField
          label="Message shown to the user"
          required
          value={inputValue(r.message)}
          onChange={(e) => set({ ...r, message: e.target.value })}
        />
        <FormField label="Show error on field">
          <Select
            value={text(r.field)}
            onChange={(field) => set({ ...r, field })}
            options={[
              { value: "", label: "Whole record" },
              ...fields.map((f) => ({
                value: text(f.key),
                label: text(f.label),
              })),
            ]}
          />
        </FormField>
      </Columns>
    </EditorForm>
  );
}
