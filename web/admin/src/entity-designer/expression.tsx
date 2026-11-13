import { useEffect, useRef, useState } from "react";
const own = (value: Record<string, any>, key: string) =>
  Object.hasOwn(value, key) ? value[key] : undefined;
export const choiceLabel = (field: Record<string, any>, key: string) =>
  typeof own(field.value_labels || {}, key) === "string"
    ? own(field.value_labels, key)
    : key;
import type { Item } from "../console-forms";
export type Expr = {
  op?: string;
  args?: Expr[];
  field?: string;
  context?: string;
  literal?: any;
  decimal?: string;
};
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
  catalog: Item;
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
    (p: Item) =>
      !contextType || contextType === "text" || p.type === contextType,
  );
  const args = value.args || [];
  const options = (catalog.operators || []).filter(
    (o: Item) => formula || o.kind !== "numeric",
  );
  const operator = options.find((x: Item) => x.key === value.op);
  const update = (i: number, v: Expr) =>
    onChange({ ...value, args: args.map((a, j) => (j === i ? v : a)) });
  return (
    <fieldset className="expression">
      <legend>{label}</legend>
      <div className="expression-head">
        <select
          aria-label={label + " value type"}
          value={mode}
          onChange={(e) => {
            const m = e.target.value;
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
            );
          }}
        >
          {!defaultOnly && <option value="field">Field</option>}
          <option value="literal">Value</option>
          <option value="decimal">Decimal</option>
          {allowContext && providers.length > 0 && (
            <option value="context">Context</option>
          )}
          {!defaultOnly && <option value="op">Expression</option>}
        </select>
        {mode === "field" ? (
          <select
            aria-label={label + " field"}
            value={value.field}
            onChange={(e) => onChange({ field: e.target.value })}
          >
            {fields.map((f) => (
              <option key={f.key} value={f.key}>
                {f.label}
              </option>
            ))}
            {transition &&
              fields.flatMap((f) =>
                ["old", "new"].map((p) => (
                  <option key={p + f.key} value={p + "." + f.key}>
                    {p === "old" ? "Previous" : "Resulting"} {f.label}
                  </option>
                )),
              )}
          </select>
        ) : mode === "context" ? (
          <select
            aria-label={label + " context"}
            value={value.context}
            onChange={(e) => onChange({ context: e.target.value })}
          >
            {providers.map((p: Item) => (
              <option key={p.key} value={p.key}>
                {p.label}
              </option>
            ))}
          </select>
        ) : mode === "op" ? (
          <select
            aria-label={label + " operator"}
            value={value.op}
            onChange={(e) => {
              const op = options.find((x: Item) => x.key === e.target.value);
              onChange({
                op: e.target.value,
                args: Array.from(
                  { length: op.min_args },
                  (_, i) => args[i] || { literal: "" },
                ),
              });
            }}
          >
            {options.map((o: Item) => (
              <option key={o.key} value={o.key}>
                {o.key}
              </option>
            ))}
          </select>
        ) : (
          <>
            <select
              aria-label={label + " literal type"}
              value={
                mode === "decimal"
                  ? "decimal"
                  : value.literal === null
                    ? "null"
                    : typeof value.literal
              }
              onChange={(e) =>
                onChange(
                  e.target.value === "decimal"
                    ? { decimal: "0" }
                    : {
                        literal:
                          e.target.value === "number"
                            ? 0
                            : e.target.value === "boolean"
                              ? true
                              : e.target.value === "object"
                                ? {}
                                : e.target.value === "null"
                                  ? null
                                  : "",
                      },
                )
              }
            >
              <option value="string">Text</option>
              <option value="number">Integer</option>
              <option value="decimal">Decimal</option>
              <option value="boolean">Boolean</option>
              <option value="null">Null</option>
              <option value="object">JSON object or list</option>
            </select>
            {value.literal !== null &&
              (typeof value.literal === "boolean" ? (
                <select
                  aria-label={label + " value"}
                  value={String(value.literal)}
                  onChange={(e) =>
                    onChange({ literal: e.target.value === "true" })
                  }
                >
                  <option>true</option>
                  <option>false</option>
                </select>
              ) : typeof value.literal === "object" ? (
                <JSONInput
                  value={value.literal}
                  label={label + " value"}
                  onChange={(v) => onChange({ literal: v })}
                />
              ) : (
                <input
                  aria-label={label + " value"}
                  value={value.decimal ?? value.literal ?? ""}
                  type={
                    mode === "decimal"
                      ? "text"
                      : typeof value.literal === "number"
                        ? "number"
                        : "text"
                  }
                  onChange={(e) =>
                    onChange(
                      mode === "decimal"
                        ? { decimal: e.target.value }
                        : {
                            literal:
                              typeof value.literal === "number"
                                ? Number(e.target.value)
                                : e.target.value,
                          },
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
              <select
                value={
                  own(value, f.key) === undefined
                    ? ""
                    : String(own(value, f.key))
                }
                onChange={(e) =>
                  onChange({
                    ...value,
                    [f.key]:
                      e.target.value === ""
                        ? undefined
                        : e.target.value === "true",
                  })
                }
              >
                <option value="">Omit</option>
                <option>true</option>
                <option>false</option>
              </select>
            ) : f.type === "choice" ? (
              <select
                value={own(value, f.key) ?? ""}
                onChange={(e) =>
                  onChange({ ...value, [f.key]: e.target.value || undefined })
                }
              >
                <option value="">Omit</option>
                {f.values?.map((v: string) => (
                  <option key={v} value={v}>
                    {choiceLabel(f, v)}
                  </option>
                ))}
              </select>
            ) : f.type === "json" ? (
              <JSONInput
                value={own(value, f.key)}
                label={f.label}
                allowEmpty
                onChange={(v) => onChange({ ...value, [f.key]: v })}
              />
            ) : (
              <input
                type={
                  f.type === "integer"
                    ? "number"
                    : f.type === "date"
                      ? "date"
                      : "text"
                }
                placeholder={f.default ? "Omit to use default" : "Omit"}
                value={own(value, f.key) ?? ""}
                onChange={(e) =>
                  onChange({
                    ...value,
                    [f.key]:
                      e.target.value === ""
                        ? undefined
                        : f.type === "integer"
                          ? Number(e.target.value)
                          : e.target.value,
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
