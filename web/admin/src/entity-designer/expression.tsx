import { useEffect, useRef, useState } from "react";
import type { EntityCatalog, Item } from "../api-types";
import "./entity.css";
// One expression shape for the SDK and the console (packages/client).
import type { Expression as Expr } from "../../../../packages/client/src/index.ts";
const own = (value: Item, key: string) =>
  Object.hasOwn(value, key) ? value[key] : undefined;
export const choiceLabel = (field: Item, key: string): string => {
  const label = own(field.value_labels || {}, key);
  return typeof label === "string" ? label : key;
};
import { Select } from "../select";
import { NumberInput } from "../number-input";
const booleanOptions = ["true", "false"].map((v) => ({ value: v, label: v }));
export function Expression({
  value,
  onChange,
  fields,
  catalog,
  transition = false,
  label = "Condition",
  depth = 0,
  formula = false,
  defaultOnly = false,
  allowContext = true,
  contextType,
}: {
  value: Expr;
  onChange: (x: Expr) => void;
  fields: Item[];
  catalog: EntityCatalog;
  transition?: boolean;
  label?: string;
  depth?: number;
  formula?: boolean;
  defaultOnly?: boolean;
  allowContext?: boolean;
  contextType?: string;
}) {
  const mode = value.op
    ? "op"
    : value.field
      ? "field"
      : value.context
        ? "context"
        : value.decimal !== undefined
          ? "decimal"
          : "literal";
  const providers = (catalog.context_providers || []).filter(
    (p) => !contextType || contextType === "text" || p.type === contextType,
  );
  const args = value.args || [];
  const options = (catalog.operators || []).filter(
    (o) => formula || o.kind !== "numeric",
  );
  const operator = options.find((x) => x.key === value.op);
  const update = (i: number, v: Expr) =>
    onChange({ ...value, args: args.map((a, j) => (j === i ? v : a)) });
  return (
    <fieldset className="expression">
      <legend>{label}</legend>
      <div className="expression-head">
        <Select
          aria-label={label + " value type"}
          value={mode}
          onChange={(m) =>
            onChange(
              m === "op"
                ? {
                    op: "eq",
                    args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                  }
                : m === "field"
                  ? { field: fields[0]?.key || "" }
                  : m === "context"
                    ? { context: providers[0]?.key || "" }
                    : m === "decimal"
                      ? { decimal: "0" }
                      : { literal: "" },
            )
          }
          options={[
            ...(!defaultOnly ? [{ value: "field", label: "Field" }] : []),
            { value: "literal", label: "Value" },
            { value: "decimal", label: "Decimal" },
            ...(allowContext && providers.length > 0
              ? [{ value: "context", label: "Context" }]
              : []),
            ...(!defaultOnly ? [{ value: "op", label: "Expression" }] : []),
          ]}
        />
        {mode === "field" ? (
          <Select
            aria-label={label + " field"}
            value={value.field}
            onChange={(field) => onChange({ field })}
            options={[
              ...fields.map((f) => ({ value: f.key, label: f.label })),
              ...(transition
                ? fields.flatMap((f) =>
                    ["old", "new"].map((p) => ({
                      value: p + "." + f.key,
                      label:
                        (p === "old" ? "Previous " : "Resulting ") + f.label,
                    })),
                  )
                : []),
            ]}
          />
        ) : mode === "context" ? (
          <Select
            aria-label={label + " context"}
            value={value.context}
            onChange={(context) => onChange({ context })}
            options={providers.map((p) => ({
              value: p.key,
              label: p.label,
            }))}
          />
        ) : mode === "op" ? (
          <Select
            aria-label={label + " operator"}
            value={value.op}
            onChange={(key) => {
              const op = options.find((x) => x.key === key);
              onChange({
                op: key,
                args: Array.from(
                  { length: op?.min_args ?? 0 },
                  (_, i) => args[i] || { literal: "" },
                ),
              });
            }}
            options={options.map((o) => ({ value: o.key, label: o.key }))}
          />
        ) : (
          <>
            <Select
              aria-label={label + " literal type"}
              value={
                mode === "decimal"
                  ? "decimal"
                  : value.literal === null
                    ? "null"
                    : typeof value.literal
              }
              onChange={(type) =>
                onChange(
                  type === "decimal"
                    ? { decimal: "0" }
                    : {
                        literal:
                          type === "number"
                            ? 0
                            : type === "boolean"
                              ? true
                              : type === "object"
                                ? {}
                                : type === "null"
                                  ? null
                                  : "",
                      },
                )
              }
              options={[
                { value: "string", label: "Text" },
                { value: "number", label: "Integer" },
                { value: "decimal", label: "Decimal" },
                { value: "boolean", label: "Boolean" },
                { value: "null", label: "Null" },
                { value: "object", label: "JSON object or list" },
              ]}
            />
            {value.literal !== null &&
              (typeof value.literal === "boolean" ? (
                <Select
                  aria-label={label + " value"}
                  value={String(value.literal)}
                  onChange={(v) => onChange({ literal: v === "true" })}
                  options={booleanOptions}
                />
              ) : typeof value.literal === "object" ? (
                <JSONInput
                  value={value.literal}
                  label={label + " value"}
                  onChange={(v) => onChange({ literal: v })}
                />
              ) : typeof value.literal === "number" ? (
                <NumberInput
                  aria-label={label + " value"}
                  value={value.literal}
                  onChange={(n) => onChange({ literal: Number(n) })}
                />
              ) : (
                <input
                  aria-label={label + " value"}
                  value={String(value.decimal ?? value.literal ?? "")}
                  inputMode={mode === "decimal" ? "decimal" : undefined}
                  onChange={(e) =>
                    onChange(
                      mode === "decimal"
                        ? { decimal: e.target.value }
                        : { literal: e.target.value },
                    )
                  }
                />
              ))}
          </>
        )}
      </div>
      {mode === "op" && depth < catalog.limits.Depth && (
        <div className="expression-args">
          {args.map((a, i) => (
            <div key={i}>
              <Expression
                value={a}
                onChange={(v) => update(i, v)}
                fields={fields}
                catalog={catalog}
                transition={transition}
                label={"Argument " + (i + 1)}
                depth={depth + 1}
                formula={formula}
                allowContext={allowContext}
              />
              {args.length > (operator?.min_args ?? 2) && (
                <button
                  className="text-button"
                  onClick={() =>
                    onChange({ ...value, args: args.filter((_, j) => j !== i) })
                  }
                >
                  Remove argument
                </button>
              )}
            </div>
          ))}
          {args.length < (operator?.max_args ?? 2) && (
            <button
              className="secondary"
              onClick={() =>
                onChange({ ...value, args: [...args, { literal: "" }] })
              }
            >
              Add argument
            </button>
          )}
        </div>
      )}
    </fieldset>
  );
}
export function FieldChoices({
  fields,
  value,
  onChange,
}: {
  fields: Item[];
  value: string[];
  onChange: (v: string[]) => void;
}) {
  return (
    <div className="field-choices">
      {fields.map((f) => (
        <label key={f.key}>
          <input
            type="checkbox"
            checked={value.includes(f.key)}
            onChange={(e) =>
              onChange(
                e.target.checked
                  ? [...value, f.key]
                  : value.filter((x) => x !== f.key),
              )
            }
          />
          {f.label}
        </label>
      ))}
    </div>
  );
}
export function ValueInputs({
  fields,
  value,
  onChange,
}: {
  fields: Item[];
  value: Item;
  onChange: (v: Item) => void;
}) {
  return (
    <div className="entity-value-inputs">
      {fields
        .filter((f) => !f.calculation)
        .map((f) => (
          <label className="field" key={f.key}>
            <span>{f.label}</span>
            {f.type === "boolean" ? (
              <Select
                value={
                  own(value, f.key) === undefined
                    ? ""
                    : String(own(value, f.key))
                }
                onChange={(v) =>
                  onChange({
                    ...value,
                    [f.key]: v === "" ? undefined : v === "true",
                  })
                }
                options={[{ value: "", label: "Omit" }, ...booleanOptions]}
              />
            ) : f.type === "choice" ? (
              <Select
                value={own(value, f.key) ?? ""}
                onChange={(v) =>
                  onChange({ ...value, [f.key]: v || undefined })
                }
                options={[
                  { value: "", label: "Omit" },
                  ...(f.values || []).map((v: string) => ({
                    value: v,
                    label: choiceLabel(f, v),
                  })),
                ]}
              />
            ) : f.type === "json" ? (
              <JSONInput
                value={own(value, f.key)}
                label={f.label}
                allowEmpty
                onChange={(v) => onChange({ ...value, [f.key]: v })}
              />
            ) : f.type === "integer" ? (
              <NumberInput
                placeholder={f.default ? "Omit to use default" : "Omit"}
                value={own(value, f.key)}
                onChange={(n) =>
                  onChange({ ...value, [f.key]: n === "" ? undefined : n })
                }
              />
            ) : (
              // Dates are typed as ISO text; the console has no native date picker.
              <input
                placeholder={
                  (f.type === "date" ? "YYYY-MM-DD, or omit" : "Omit") +
                  (f.default ? " to use default" : "")
                }
                pattern={f.type === "date" ? "\\d{4}-\\d{2}-\\d{2}" : undefined}
                inputMode={f.type === "date" ? "numeric" : undefined}
                value={own(value, f.key) ?? ""}
                onChange={(e) =>
                  onChange({
                    ...value,
                    [f.key]: e.target.value === "" ? undefined : e.target.value,
                  })
                }
              />
            )}
          </label>
        ))}
    </div>
  );
}

function JSONInput({
  value,
  onChange,
  label,
  allowEmpty = false,
}: {
  value: unknown;
  onChange: (v: unknown) => void;
  label: string;
  allowEmpty?: boolean;
}) {
  const serialize = (v: unknown) =>
    v === undefined ? "" : JSON.stringify(v, null, 2);
  const [text, setText] = useState(() => serialize(value));
  const last = useRef<string | undefined>(JSON.stringify(value));
  useEffect(() => {
    const next = JSON.stringify(value);
    if (next !== last.current) {
      last.current = next;
      setText(serialize(value));
    }
  }, [value]);
  return (
    <textarea
      aria-label={label}
      rows={5}
      value={text}
      placeholder={allowEmpty ? "Omit, or enter a JSON object or list" : ""}
      onChange={(e) => {
        const raw = e.target.value;
        setText(raw);
        if (!raw.trim() && allowEmpty) {
          last.current = undefined;
          e.currentTarget.setCustomValidity("");
          onChange(undefined);
          return;
        }
        try {
          const parsed: unknown = JSON.parse(raw);
          if (parsed === null || typeof parsed !== "object") throw new Error();
          last.current = JSON.stringify(parsed);
          e.currentTarget.setCustomValidity("");
          onChange(parsed);
        } catch {
          e.currentTarget.setCustomValidity(
            "Enter a valid JSON object or list.",
          );
        }
      }}
    />
  );
}
