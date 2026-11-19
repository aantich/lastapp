import { Plus } from "lucide-react";
import { Badge, Button, EmptyState, GridTable, Inline, Panel } from "../ui";
import { isItem, items, num, text, type Item } from "../api-types";
import { RowActions } from "./editor-dialog";
import {
  choiceLabel,
  definitionOf,
  describe,
  strings,
  type Designer,
} from "./shared";

const uniqueNotes: Record<string, string> = {
  entity: "Unique",
  owner: "Unique per owner",
  team: "Unique per team",
};

// A number bound as the backend stores it (a JSON number or decimal string).
const bound = (v: unknown) =>
  typeof v === "number" || typeof v === "string" ? String(v) : "";

// What a field does beyond its type: its calculation, choices, default,
// bounds, uniqueness and length, all from the field definition.
function behaviour(f: Item, fields: Item[]) {
  if (f.calculation) return describe(f.calculation, fields);
  const values = strings(f.values);
  if (values.length) return values.map((v) => choiceLabel(f, v)).join(" · ");
  const min = bound(f.minimum),
    max = bound(f.maximum);
  const isCreator =
    isItem(f.default) && f.default.context === "actor.id" && !f.default.op;
  const notes = [
    isCreator
      ? "Set to creator"
      : f.default && "Default " + describe(f.default, fields),
    min && max
      ? `between ${min} and ${max}`
      : min
        ? "at least " + min
        : max && "at most " + max,
    uniqueNotes[text(f.unique)],
    f.type === "reference" && "Options filtered by access",
    f.type === "text" &&
      !!num(f.max_length) &&
      `Up to ${num(f.max_length)} characters`,
  ].filter(Boolean);
  return notes.join(" · ") || "—";
}

// The Fields tab: the draft's fields in the definition grid.
export function FieldsTab({ d }: { d: Designer }) {
  const { catalog, draft, entities, record, remove, setEditField } = d;
  const fields = items(draft.fields);
  const published = items(definitionOf(record?.active_definition)?.fields);
  const owner = text(draft.owner_field);
  return (
    <Panel
      flush
      title="Fields"
      actions={
        <Button
          size="sm"
          icon={<Plus size={15} aria-hidden />}
          onClick={() =>
            setEditField({
              key: "",
              label: "",
              type: catalog.types[0],
              max_length: 200,
            })
          }
        >
          Add field
        </Button>
      }
    >
      {fields.length ? (
        <GridTable
          label="Fields"
          columns={[
            { key: "field", header: "Field", track: "minmax(150px,1.4fr)" },
            { key: "type", header: "Type", track: "minmax(120px,1fr)" },
            { key: "required", header: "Required", track: "90px" },
            {
              key: "behaviour",
              header: "Behaviour",
              track: "minmax(160px,2fr)",
              muted: true,
            },
            {
              key: "actions",
              header: "Actions",
              hideHeader: true,
              track: "76px",
            },
          ]}
          rows={fields.map((f) => {
            const key = text(f.key),
              isNew =
                !!record?.active_version_id &&
                !published.some((p) => p.key === key);
            return {
              key,
              highlight: isNew,
              cells: {
                field: (
                  <Inline gap="sm">
                    <code>{key}</code>
                    {isNew && (
                      <Badge pill tone="info">
                        new
                      </Badge>
                    )}
                  </Inline>
                ),
                type:
                  f.type === "reference"
                    ? "reference → " +
                      (text(entities.find((e) => e.id === f.target)?.label) ||
                        text(f.target))
                    : f.calculation
                      ? text(f.type) + " · calculated"
                      : text(f.type),
                required:
                  owner === key
                    ? "Auto"
                    : f.calculation
                      ? "—"
                      : f.required
                        ? "Yes"
                        : "No",
                behaviour: behaviour(f, fields),
                actions: (
                  <RowActions
                    title={text(f.label)}
                    edit={() => setEditField({ ...f })}
                    remove={() => remove("fields", key)}
                  />
                ),
              },
            };
          })}
        />
      ) : (
        <EmptyState compact title="No fields yet">
          Add your first field to start defining this entity.
        </EmptyState>
      )}
    </Panel>
  );
}
