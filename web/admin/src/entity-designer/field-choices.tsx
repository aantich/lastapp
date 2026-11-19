import type { ReactNode } from "react";
import { text, type Item } from "../api-types";
import { Checkbox, Columns, FieldGroup } from "../ui";

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
        <Columns min="xs" gap="sm">
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
