import { Checkbox, FieldGroup } from "../ui";
import { text, type EntityCatalog, type Item } from "../api-types";
import { Expression } from "./expression";
import { FieldChoices } from "./field-choices";
import { isExpression, strings } from "./shared";

// The path of one permission: the fields it reaches together with the
// record condition (and, for updates, the change) they are reached under.
export function PermissionPath({
  p,
  patch,
  fields,
  catalog,
}: {
  p: Item;
  patch: (v: Item) => void;
  fields: Item[];
  catalog: EntityCatalog;
}) {
  const withFields = ["read", "create", "update"].includes(text(p.operation));
  return (
    <FieldGroup
      legend="Path"
      description={
        withFields
          ? "The fields this rule reaches and the records they are on. New fields never join this list automatically."
          : "The records this rule reaches."
      }
      framed
    >
      {withFields && (
        <FieldChoices
          legend="Which fields"
          fields={fields.filter(
            (f) => p.operation === "read" || !f.calculation,
          )}
          value={strings(p.fields)}
          onChange={(v) => patch({ fields: v })}
          empty="Add fields to this entity first."
        />
      )}
      <Checkbox
        label="Limit to records matching a condition"
        checked={!!p.when}
        onChange={(on) =>
          patch({
            when: on
              ? {
                  op: "eq",
                  args: [{ field: fields[0]?.key || "" }, { literal: "" }],
                }
              : undefined,
          })
        }
      />
      {isExpression(p.when) && (
        <Expression
          value={p.when}
          onChange={(v) => patch({ when: v })}
          fields={fields}
          catalog={catalog}
        />
      )}
      {p.operation === "update" && (
        <>
          <Checkbox
            label="Authorize a particular change"
            checked={!!p.change_when}
            onChange={(on) =>
              patch({
                change_when: on
                  ? {
                      op: "eq",
                      args: [
                        { field: "old." + (fields[0]?.key || "") },
                        { literal: "" },
                      ],
                    }
                  : undefined,
              })
            }
          />
          {isExpression(p.change_when) && (
            <Expression
              value={p.change_when}
              onChange={(v) => patch({ change_when: v })}
              fields={fields}
              catalog={catalog}
              label="Change"
              transition
            />
          )}
        </>
      )}
    </FieldGroup>
  );
}
