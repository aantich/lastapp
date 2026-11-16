import { useState } from "react";
import { Select } from "../select";
import type { EntityCatalog, Item } from "../api-types";
import { Expression } from "./expression";
import { keyOf } from "./shared";

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
    <form
      onSubmit={(e) => {
        e.preventDefault();
        save(r);
      }}
    >
      <label className="field">
        <span>Name</span>
        <input
          autoFocus
          required
          value={r.name || ""}
          onChange={(e) =>
            set({
              ...r,
              name: e.target.value,
              ...(!value.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
      </label>
      <label className="field">
        <span>Rule type</span>
        <Select
          value={r.kind}
          onChange={(kind) => set({ ...r, kind })}
          options={[
            { value: "validation", label: "Validate resulting record" },
            { value: "change", label: "Allowed change" },
          ]}
        />
      </label>
      <label>
        <input
          type="checkbox"
          checked={!!r.when}
          onChange={(e) =>
            set({
              ...r,
              when: e.target.checked
                ? {
                    op: "eq",
                    args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                  }
                : undefined,
            })
          }
        />{" "}
        Apply only when
      </label>
      {r.when && (
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
      <Expression
        value={r.assert}
        onChange={(v) => set({ ...r, assert: v })}
        fields={fields}
        catalog={catalog}
        transition={r.kind === "change"}
        label="Must be true"
        allowContext={false}
      />
      <label className="field">
        <span>Message shown to the user</span>
        <input
          required
          value={r.message}
          onChange={(e) => set({ ...r, message: e.target.value })}
        />
      </label>
      <label className="field">
        <span>Show error on field</span>
        <Select
          value={r.field || ""}
          onChange={(field) => set({ ...r, field })}
          options={[
            { value: "", label: "Whole record" },
            ...fields.map((f) => ({ value: f.key, label: f.label })),
          ]}
        />
      </label>
      <button className="primary">Save rule</button>
    </form>
  );
}
