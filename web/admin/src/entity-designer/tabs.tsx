import { Plus } from "lucide-react";
import { api } from "../api";
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  Checkbox,
  ChipRow,
  Columns,
  DataTable,
  Disclosure,
  EmptyState,
  FormField,
  GridTable,
  Inline,
  Panel,
  PathCard,
  Select,
  Stack,
  StatusBadge,
  TextField,
  confirmAction,
} from "../ui";
import { isItem, items, num, text, type Item } from "../api-types";
import { FieldChoices } from "./expression";
import { RowActions } from "./editor-dialog";
import {
  choiceLabel,
  ruleRows,
  definitionOf,
  describe,
  inputValue,
  strings,
  whoOf,
  type Designer,
} from "./shared";

const fieldOptions = (fields: Item[], type: string) => [
  { value: "", label: "None" },
  ...fields
    .filter((f) => f.type === type)
    .map((f) => ({ value: text(f.key), label: text(f.label) })),
];

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
  if (f.calculation) return <code>{describe(f.calculation, fields)}</code>;
  const values = strings(f.values);
  if (values.length)
    return (
      <Inline gap="sm">
        {values.map((v) => (
          <Badge chip key={v}>
            {choiceLabel(f, v)}
          </Badge>
        ))}
      </Inline>
    );
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
                      <Badge chip tone="info">
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

// Entity-wide settings: name, ownership, default fields, sharing and the
// lifecycle (archive/restore).
export function EntitySettings({ d }: { d: Designer }) {
  const { busy, dirty, draft, record, run, save, setDraft, setRecord } = d;
  const fields = items(draft.fields),
    archived = record?.state === "archived";
  return (
    <Card>
      <CardBody>
        <Disclosure
          summary="Entity settings"
          meta="Name, ownership, default fields and sharing"
        >
          <Stack gap="md">
            <Columns min={200} gap="md">
              <TextField
                label="Name"
                value={inputValue(draft.label)}
                onChange={(e) => setDraft({ ...draft, label: e.target.value })}
              />
              <FormField label="Owner field">
                <Select
                  value={text(draft.owner_field)}
                  onChange={(owner_field) =>
                    setDraft({ ...draft, owner_field })
                  }
                  options={fieldOptions(fields, "user")}
                />
              </FormField>
              <FormField label="Owning team field">
                <Select
                  value={text(draft.team_field)}
                  onChange={(team_field) => setDraft({ ...draft, team_field })}
                  options={fieldOptions(fields, "team")}
                />
              </FormField>
            </Columns>
            <FieldChoices
              legend="Default fields"
              description="Included when an App doesn't select specific fields."
              fields={fields}
              value={strings(draft.default_projection)}
              onChange={(v) => setDraft({ ...draft, default_projection: v })}
              empty="Add fields to choose the defaults."
            />
            <Checkbox
              label="Allow record sharing"
              checked={!!draft.sharing}
              onChange={(sharing) => setDraft({ ...draft, sharing })}
            />
            <Inline gap="md">
              <Button
                variant="danger"
                disabled={busy || !!record?.pending_version_id}
                onClick={async () => {
                  if (
                    await confirmAction(
                      archived
                        ? {
                            title: "Restore entity?",
                            message: "Restore this entity?",
                            confirmLabel: "Restore",
                          }
                        : {
                            title: "Archive entity?",
                            message:
                              "Archive this entity? Its API will stop serving requests; data is preserved.",
                            confirmLabel: "Archive",
                            danger: true,
                          },
                    )
                  )
                    void run(async () => {
                      const saved = dirty ? await save() : record!;
                      const v = await api<Item>(
                        "/v1/entities/" + saved.id,
                        "PATCH",
                        {
                          definition: draft,
                          state: archived ? "active" : "archived",
                        },
                        num(saved.revision),
                      );
                      setRecord(v);
                    });
                }}
              >
                {archived ? "Restore entity" : "Archive entity"}
              </Button>
            </Inline>
          </Stack>
        </Disclosure>
      </CardBody>
    </Card>
  );
}

// Fields no read path reaches. New fields never join an existing allowlist,
// so the author adds them to a path explicitly.
const unreached = (draft: Item) => {
  const read = new Set(
    items(draft.permissions)
      .filter((p) => p.operation === "read")
      .flatMap((p) => strings(p.fields)),
  );
  return items(draft.fields).filter((f) => !read.has(text(f.key)));
};

const fieldOperations = ["read", "create", "update"];

// The Access tab: permission paths, each one unit of who, which operation,
// which fields and which records. Everything else is denied.
export function AccessTab({ d }: { d: Designer }) {
  const { draft, remove, roles, setDraft, setEditPermission } = d;
  const fields = items(draft.fields),
    permissions = items(draft.permissions),
    missing = unreached(draft),
    target = permissions.find((p) => p.operation === "read");
  return (
    <Panel
      title="Permission paths"
      lead="Fields and their record predicates stay together. Apps can only narrow them. Administrative roles grant no data access."
      actions={
        <Button
          size="sm"
          icon={<Plus size={15} aria-hidden />}
          onClick={() =>
            setEditPermission({
              key: "",
              label: "",
              operation: "read",
              relationship: "member",
              fields: [],
            })
          }
        >
          Add access rule
        </Button>
      }
    >
      {permissions.length ? (
        permissions.map((p) => {
          const keys = strings(p.fields);
          return (
            <PathCard
              key={text(p.key)}
              who={whoOf(p, roles)}
              whoTone={
                strings(p.roles).length ||
                p.relationship === "user" ||
                p.relationship === "team"
                  ? "inverse"
                  : p.relationship === "owner"
                    ? "success"
                    : "neutral"
              }
              operations={[text(p.operation)]}
              conditions={[
                p.when && "where " + describe(p.when, fields),
                p.change_when &&
                  "change when " + describe(p.change_when, fields),
              ].filter((c): c is string => !!c)}
              fields={
                fieldOperations.includes(text(p.operation))
                  ? keys.length
                    ? "Fields: " + keys.join(", ")
                    : "Fields: none (record only)"
                  : "Whole record"
              }
              aside={
                <>
                  {text(p.label)}
                  <RowActions
                    title={text(p.label)}
                    edit={() => setEditPermission({ ...p })}
                    remove={() => remove("permissions", text(p.key))}
                  />
                </>
              }
            />
          );
        })
      ) : (
        <EmptyState compact title="No access yet">
          Add a rule by choosing who, what they can do, and which fields. Access
          is denied until a path matches.
        </EmptyState>
      )}
      {permissions.length > 0 && missing.length > 0 && (
        <Callout
          title={`${missing.map((f) => text(f.key)).join(", ")} ${
            missing.length === 1 ? "is" : "are"
          } not in any read path yet`}
          action={
            target && (
              <Button
                size="sm"
                onClick={() =>
                  setDraft({
                    ...draft,
                    permissions: permissions.map((p) =>
                      p === target
                        ? {
                            ...p,
                            fields: [
                              ...strings(p.fields),
                              ...missing.map((f) => text(f.key)),
                            ],
                          }
                        : p,
                    ),
                  })
                }
              >
                Add to {whoOf(target, roles)}
              </Button>
            )
          }
        >
          New fields never join existing allowlists automatically.
        </Callout>
      )}
    </Panel>
  );
}

// The Rules tab: defaults, calculations, validation and allowed changes.
export function RulesTab({ d }: { d: Designer }) {
  const { draft, setEditRule } = d;
  const fields = items(draft.fields),
    rows = ruleRows(d);
  return (
    <Panel
      title="Rules"
      lead="Rules apply to every write, from every App. Set defaults and calculations in a field's editor."
      actions={
        <Button
          size="sm"
          icon={<Plus size={15} aria-hidden />}
          onClick={() =>
            setEditRule({
              key: "",
              name: "",
              kind: "validation",
              assert: {
                op: "is_not_null",
                args: [{ field: fields[0]?.key || "" }],
              },
              message: "",
            })
          }
        >
          Add rule
        </Button>
      }
    >
      {rows.length ? (
        rows.map((r) => (
          <ChipRow
            key={r.key}
            chip={r.kind}
            expression={r.expression}
            note={r.note}
            actions={
              <RowActions title={r.title} edit={r.edit} remove={r.remove} />
            }
          />
        ))
      ) : (
        <EmptyState compact title="No rules yet">
          Add validation or allowed-change rules here. Set defaults and
          calculations in a field's editor.
        </EmptyState>
      )}
    </Panel>
  );
}

// The History tab: the entity's recent publications (the backend returns
// the latest 20), newest first.
export function HistoryTab({ d }: { d: Designer }) {
  const publications = items(d.record?.publications);
  return (
    <Panel
      title="Publications"
      lead="Each publication is an immutable version. Apps use the active one."
    >
      {publications.length ? (
        <DataTable
          caption="Publication history"
          rows={publications}
          rowKey={(p) => text(p.id)}
          columns={[
            {
              key: "started",
              header: "Started",
              render: (p) => new Date(text(p.created_at)).toLocaleString(),
            },
            {
              key: "version",
              header: "Version",
              render: (p) => (
                <Inline gap="sm">
                  <Badge mono title={text(p.id)}>
                    {text(p.id).slice(0, 8)}
                  </Badge>
                  {p.id === d.record?.active_version_id && (
                    <Badge tone="success">Active</Badge>
                  )}
                </Inline>
              ),
            },
            {
              key: "state",
              header: "State",
              render: (p) => <StatusBadge state={p.state} />,
            },
            {
              key: "error",
              header: "Error",
              render: (p) => text(p.error_message) || "—",
            },
          ]}
        />
      ) : (
        <EmptyState compact title="Not published yet">
          Review changes and publish to create the first version.
        </EmptyState>
      )}
    </Panel>
  );
}
