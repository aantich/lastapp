import { text, type Item } from "../api-types";
import {
  Button,
  Columns,
  DatePicker,
  FormField,
  Inline,
  NumberInput,
  Select,
  Stack,
  TextField,
} from "../ui";
import { booleanOptions } from "./boolean-options";
import { JSONInput } from "./json-input";
import { choiceLabel, inputValue, strings } from "./shared";
const own = (value: Item, key: string) =>
  Object.hasOwn(value, key) ? value[key] : undefined;

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
