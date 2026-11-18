import { useEffect, useRef, useState, type ReactNode } from "react";
import { text, type EntityCatalog, type Item } from "../api-types";
import { useListKeys } from "../list-keys";
import { choiceLabel, inputValue, strings } from "./shared";
// One expression shape for the SDK and the console (packages/client).
import type { Expression as Expr } from "../../../../packages/client/src/index.ts";
const own = (value: Item, key: string) =>
  Object.hasOwn(value, key) ? value[key] : undefined;
import {
  Button,
  Checkbox,
  Columns,
  DatePicker,
  FieldGroup,
  FormField,
  Inline,
  NumberInput,
  Select,
  Stack,
  TextField,
  Textarea,
} from "../ui";
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
  const argKeys = useListKeys(args.length);
  const options = (catalog.operators || []).filter(
    (o) => formula || o.kind !== "numeric",
  );
  const operator = options.find((x) => x.key === value.op);
  const update = (i: number, v: Expr) =>
    onChange({ ...value, args: args.map((a, j) => (j === i ? v : a)) });
  return (
    <FieldGroup legend={label} nested={depth > 0}>
      <Columns min={160} gap="sm">
        <Select
          aria-label={label + " value type"}
          value={mode}
          onChange={(m) =>
            onChange(
              m === "op"
                ? {
                    op: "eq",
                    args: [{ field: text(fields[0]?.key) }, { literal: "" }],
                  }
                : m === "field"
                  ? { field: text(fields[0]?.key) }
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
              ...fields.map((f) => ({
                value: text(f.key),
                label: text(f.label),
              })),
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
                  hideLabel
                  onChange={(v) => onChange({ literal: v })}
                />
              ) : typeof value.literal === "number" ? (
                <NumberInput
                  aria-label={label + " value"}
                  value={value.literal}
                  onChange={(n) => onChange({ literal: Number(n) })}
                />
              ) : (
                <TextField
                  hideLabel
                  label={label + " value"}
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
      </Columns>
      {mode === "op" && depth < catalog.limits.Depth && (
        <Stack gap="md">
          {args.map((a, i) => (
            <Stack gap="sm" key={argKeys.keys[i]}>
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
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => {
                    argKeys.remove(i);
                    onChange({
                      ...value,
                      args: args.filter((_, j) => j !== i),
                    });
                  }}
                >
                  Remove argument
                </Button>
              )}
            </Stack>
          ))}
          {args.length < (operator?.max_args ?? 2) && (
            <Inline>
              <Button
                size="sm"
                onClick={() =>
                  onChange({ ...value, args: [...args, { literal: "" }] })
                }
              >
                Add argument
              </Button>
            </Inline>
          )}
        </Stack>
      )}
    </FieldGroup>
  );
}
// A choice of fields (or of other named items: roles, permissions) as a
// group of checkboxes under one legend.
export function FieldChoices({
  legend,
  description,
  fields,
  value,
  onChange,
  empty,
}: {
  legend: string;
  description?: ReactNode;
  fields: Item[];
  value: string[];
  onChange: (v: string[]) => void;
  empty?: ReactNode;
}) {
  return (
    <FieldGroup legend={legend} description={description}>
      {fields.length ? (
        <Columns min={160} gap="sm">
          {fields.map((f) => {
            const key = text(f.key);
            return (
              <Checkbox
                key={key}
                label={text(f.label)}
                checked={value.includes(key)}
                onChange={(checked) =>
                  onChange(
                    checked ? [...value, key] : value.filter((x) => x !== key),
                  )
                }
              />
            );
          })}
        </Columns>
      ) : (
        <p>{empty || "Nothing to choose yet."}</p>
      )}
    </FieldGroup>
  );
}
// Synthetic values for the test scenario, one input per stored field.
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
    <Columns min={220} gap="md">
      {fields
        .filter((f) => !f.calculation)
        .map((f) => {
          const key = text(f.key),
            label = text(f.label),
            current = own(value, key),
            omit = f.default ? "Omit to use default" : "Omit";
          const set = (v: unknown) => onChange({ ...value, [key]: v });
          return f.type === "boolean" ? (
            <FormField label={label} key={key}>
              <Select
                value={current === undefined ? "" : String(current)}
                onChange={(v) => set(v === "" ? undefined : v === "true")}
                options={[{ value: "", label: "Omit" }, ...booleanOptions]}
              />
            </FormField>
          ) : f.type === "choice" ? (
            <FormField label={label} key={key}>
              <Select
                value={text(current)}
                onChange={(v) => set(v || undefined)}
                options={[
                  { value: "", label: "Omit" },
                  ...strings(f.values).map((v) => ({
                    value: v,
                    label: choiceLabel(f, v),
                  })),
                ]}
              />
            </FormField>
          ) : f.type === "json" ? (
            <JSONInput
              key={key}
              value={current}
              label={label}
              allowEmpty
              onChange={set}
            />
          ) : f.type === "integer" ? (
            <FormField label={label} key={key}>
              <NumberInput
                placeholder={omit}
                value={inputValue(current)}
                onChange={(n) => set(n === "" ? undefined : n)}
              />
            </FormField>
          ) : f.type === "date" ? (
            <Stack gap="sm" key={key}>
              <FormField label={label}>
                <DatePicker
                  placeholder={omit}
                  value={text(current)}
                  onChange={(v) => set(v || undefined)}
                />
              </FormField>
              {current !== undefined && (
                <Inline>
                  <Button
                    variant="link"
                    size="sm"
                    onClick={() => set(undefined)}
                  >
                    Omit {label}
                  </Button>
                </Inline>
              )}
            </Stack>
          ) : (
            <TextField
              key={key}
              label={label}
              placeholder={omit}
              value={inputValue(current) ?? ""}
              onChange={(e) =>
                set(e.target.value === "" ? undefined : e.target.value)
              }
            />
          );
        })}
    </Columns>
  );
}

function JSONInput({
  value,
  onChange,
  label,
  hideLabel,
  allowEmpty = false,
}: {
  value: unknown;
  onChange: (v: unknown) => void;
  label: string;
  hideLabel?: boolean;
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
    <Textarea
      label={label}
      hideLabel={hideLabel}
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
