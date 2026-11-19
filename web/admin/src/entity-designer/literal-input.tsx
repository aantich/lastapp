import { NumberInput, Select, TextField } from "../ui";
import { booleanOptions } from "./boolean-options";
import { JSONInput } from "./json-input";
// One expression shape for the SDK and the console (packages/client).
import type { Expression as Expr } from "../../../../packages/client/src/index.ts";

// The literal (or decimal) end of an expression: its type, then its value.
export function LiteralInput({
  value,
  onChange,
  mode,
  label,
}: {
  value: Expr;
  onChange: (x: Expr) => void;
  mode: string;
  label: string;
}) {
  return (
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
  );
}
