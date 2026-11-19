import { text, type EntityCatalog, type Item } from "../api-types";
import { useListKeys } from "../list-keys";
import { Button, Columns, FieldGroup, Inline, Select, Stack } from "../ui";
import { LiteralInput } from "./literal-input";
// One expression shape for the SDK and the console (packages/client).
import type { Expression as Expr } from "../../../../packages/client/src/index.ts";
// FieldChoices and ValueInputs keep this module path for their importers.
export { FieldChoices } from "./field-choices";
export { ValueInputs } from "./value-inputs";
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
          <LiteralInput
            value={value}
            onChange={onChange}
            mode={mode}
            label={label}
          />
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
