import { useState } from "react";
import { RemoteSelect } from "../remote-select";
import { Select } from "../select";
import { NumberInput } from "../number-input";
import type { EntityCatalog, Item } from "../api-types";
import { Expression } from "./expression";
import { choiceLabel, fieldLabel, keyOf } from "./shared";

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
    [choice, setChoice] = useState<string>((field.values || []).join(", "));
  const patch = (v: Item) => set({ ...f, ...v });
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (fields.some((x) => x.key === f.key && x.key !== field.key)) {
          setError(
            "Another field already uses this key. Choose a different key.",
          );
          return;
        }
        const value = { ...f };
        if (value.type === "choice")
          value.values = choice
            .split(",")
            .map((v) => v.trim())
            .filter(Boolean);
        if (value.type === "choice" && value.value_labels)
          value.value_labels = Object.fromEntries(
            Object.entries(value.value_labels).filter(([key]) =>
              value.values.includes(key),
            ),
          );
        save(value);
      }}
    >
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <label className="field">
        <span>Name</span>
        <input
          autoFocus
          required
          value={f.label}
          onChange={(e) =>
            patch({
              label: e.target.value,
              ...(!field.key ? { key: keyOf(e.target.value) } : {}),
            })
          }
        />
      </label>
      <label className="field">
        <span>Type</span>
        <Select
          value={f.type}
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
      </label>
      <label>
        <input
          type="checkbox"
          checked={!!f.required}
          onChange={(e) => patch({ required: e.target.checked })}
        />
        Required
      </label>
      {f.type === "text" && (
        <label className="field">
          <span>Maximum length</span>
          <NumberInput
            required
            max={catalog.field_constraints.max_text_length}
            min={catalog.field_constraints.min_text_length}
            value={f.max_length}
            onChange={(n) => patch({ max_length: Number(n) })}
          />
        </label>
      )}
      {f.type === "decimal" && (
        <div className="entity-toolbar">
          {["precision", "scale"].map((key) => (
            <label className="field" key={key}>
              <span>{fieldLabel(key)}</span>
              <NumberInput
                required
                value={f[key]}
                onChange={(n) => patch({ [key]: Number(n) })}
              />
            </label>
          ))}
        </div>
      )}
      {f.type === "choice" && (
        <label className="field">
          <span>Choices, separated by commas</span>
          <input
            required
            value={choice}
            onChange={(e) => setChoice(e.target.value)}
          />
        </label>
      )}
      {f.type === "choice" && (
        <details>
          <summary>Choice display labels</summary>
          <p className="muted">Keep keys stable; change their labels here.</p>
          {choice
            .split(",")
            .map((v) => v.trim())
            .filter(Boolean)
            .map((key) => (
              <label className="field" key={key}>
                <span>{key}</span>
                <input
                  value={choiceLabel(f, key)}
                  onChange={(e) =>
                    patch({
                      value_labels: {
                        ...f.value_labels,
                        [key]: e.target.value,
                      },
                    })
                  }
                />
              </label>
            ))}
        </details>
      )}
      {f.type === "reference" && (
        <label className="field">
          <span>Related entity</span>
          <RemoteSelect
            path={"/v1/spaces/" + space + "/entities"}
            label="Related entity"
            value={f.target || ""}
            initial={[]}
            labelKey="label"
            onChange={(value) => patch({ target: value })}
          />
        </label>
      )}
      {["integer", "decimal"].includes(f.type) && (
        <div className="entity-toolbar">
          {["minimum", "maximum"].map((key) => (
            <label className="field" key={key}>
              <span>{fieldLabel(key)}</span>
              <input
                value={f[key] ?? ""}
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
            </label>
          ))}
        </div>
      )}
      <details>
        <summary>Defaults, calculation and query options</summary>
        <label className="field">
          <span>Value source</span>
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
        </label>
        {(f.default || f.calculation) && (
          <Expression
            value={f.calculation || f.default}
            onChange={(value) =>
              patch(f.calculation ? { calculation: value } : { default: value })
            }
            fields={fields}
            catalog={catalog}
            label={f.calculation ? "Formula" : "Default"}
            formula={!!f.calculation}
            defaultOnly={!!f.default}
            contextType={f.default ? f.type : undefined}
            allowContext={!f.calculation}
          />
        )}
        {["filterable", "sortable"].map((key) => (
          <label key={key}>
            <input
              type="checkbox"
              checked={!!f[key]}
              onChange={(e) => patch({ [key]: e.target.checked })}
            />
            {key === "filterable" ? "Allow filtering" : "Allow sorting"}
          </label>
        ))}
        <label className="field">
          <span>Uniqueness</span>
          <Select
            value={f.unique || ""}
            onChange={(unique) => patch({ unique })}
            options={[
              { value: "", label: "None" },
              { value: "entity", label: "Across this entity" },
              { value: "owner", label: "Per owner" },
              { value: "team", label: "Per team" },
            ]}
          />
        </label>
        <label className="field">
          <span>Stable key</span>
          <input
            required
            pattern="[a-z][a-z0-9_]*"
            value={f.key}
            disabled={!!field.key}
            onChange={(e) => patch({ key: e.target.value })}
          />
        </label>
      </details>
      <button className="primary">Save field</button>
    </form>
  );
}
