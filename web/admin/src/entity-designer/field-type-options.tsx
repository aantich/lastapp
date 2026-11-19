import {
  Columns,
  Disclosure,
  FormField,
  NumberInput,
  RemoteSelect,
  Stack,
  TextField,
} from "../ui";
import { isItem, text, type EntityCatalog, type Item } from "../api-types";
import { choiceLabel, fieldLabel, inputValue } from "./shared";

// The settings only some field types have: length, precision, choices,
// related entity and numeric bounds.
export function FieldTypeOptions({
  f,
  patch,
  catalog,
  space,
  choice,
  setChoice,
  choices,
}: {
  f: Item;
  patch: (v: Item) => void;
  catalog: EntityCatalog;
  space: string;
  choice: string;
  setChoice: (v: string) => void;
  choices: string[];
}) {
  return (
    <>
      {f.type === "text" && (
        <FormField label="Maximum length" required>
          <NumberInput
            required
            max={catalog.field_constraints.max_text_length}
            min={catalog.field_constraints.min_text_length}
            value={inputValue(f.max_length)}
            onChange={(n) => patch({ max_length: Number(n) })}
          />
        </FormField>
      )}
      {f.type === "decimal" && (
        <Columns min="xs" gap="md">
          {["precision", "scale"].map((key) => (
            <FormField label={fieldLabel(key)} required key={key}>
              <NumberInput
                required
                value={inputValue(f[key])}
                onChange={(n) => patch({ [key]: Number(n) })}
              />
            </FormField>
          ))}
        </Columns>
      )}
      {f.type === "choice" && (
        <TextField
          label="Choices, separated by commas"
          required
          value={choice}
          onChange={(e) => setChoice(e.target.value)}
        />
      )}
      {f.type === "choice" && choices.length > 0 && (
        <Disclosure summary="Choice display labels">
          <Stack gap="md">
            <p>Keep keys stable; change their labels here.</p>
            <Columns min="sm" gap="md">
              {choices.map((key) => (
                <TextField
                  key={key}
                  label={key}
                  value={choiceLabel(f, key)}
                  onChange={(e) =>
                    patch({
                      value_labels: {
                        ...(isItem(f.value_labels) ? f.value_labels : {}),
                        [key]: e.target.value,
                      },
                    })
                  }
                />
              ))}
            </Columns>
          </Stack>
        </Disclosure>
      )}
      {f.type === "reference" && (
        <FormField label="Related entity">
          <RemoteSelect
            path={"/v1/spaces/" + space + "/entities"}
            label="Related entity"
            value={text(f.target)}
            initial={[]}
            labelKey="label"
            onChange={(value) => patch({ target: value })}
          />
        </FormField>
      )}
      {["integer", "decimal"].includes(text(f.type)) && (
        <Columns min="xs" gap="md">
          {["minimum", "maximum"].map((key) => (
            <TextField
              key={key}
              label={fieldLabel(key)}
              inputMode={f.type === "integer" ? "numeric" : "decimal"}
              value={inputValue(f[key]) ?? ""}
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
          ))}
        </Columns>
      )}
    </>
  );
}
