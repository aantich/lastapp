import { Plus } from "lucide-react";
import { api } from "../api";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  Columns,
  DataTable,
  Disclosure,
  EmptyState,
  FormField,
  Inline,
  Select,
  Stack,
  StatusBadge,
  TextField,
  confirmAction,
} from "../ui";
import { items, num, text, type Item } from "../api-types";
import { FieldChoices } from "./expression";
import { RowActions } from "./editor-dialog";
import {
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

// The Fields tab: the draft's fields and entity-wide settings.
export function FieldsTab({ d }: { d: Designer }) {
  const { catalog, draft, record, remove, setDraft, setEditField } = d;
  const fields = items(draft.fields);
  const published = items(definitionOf(record?.active_definition)?.fields);
  return (
    <Stack>
      <Card>
        <CardHeader
          level={3}
          title="Fields"
          description="Choose the information this entity stores."
        />
        <CardBody>
          <Stack gap="md">
            <Inline align="end" gap="md">
              <TextField
                label="Name"
                value={inputValue(draft.label)}
                onChange={(e) => setDraft({ ...draft, label: e.target.value })}
              />
              <Button
                variant="primary"
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
            </Inline>
            {fields.length ? (
              <DataTable
                caption="Fields"
                rows={fields}
                rowKey={(f) => text(f.key)}
                columns={[
                  {
                    key: "field",
                    header: "Field",
                    render: (f) => (
                      <Inline gap="sm">
                        <strong>{text(f.label)}</strong>
                        <Badge mono>{text(f.key)}</Badge>
                        {!!record?.active_version_id &&
                          !published.some((p) => p.key === f.key) && (
                            <Badge tone="info">New</Badge>
                          )}
                      </Inline>
                    ),
                  },
                  {
                    key: "type",
                    header: "Type",
                    render: (f) => <Badge mono>{text(f.type)}</Badge>,
                  },
                  {
                    key: "required",
                    header: "Required",
                    render: (f) => (f.required ? "Yes" : "No"),
                  },
                  {
                    key: "behaviour",
                    header: "Behaviour",
                    render: (f) =>
                      [
                        f.calculation &&
                          "Calculated: " + describe(f.calculation, fields),
                        f.default && "Default: " + describe(f.default, fields),
                        f.unique && "Unique per " + text(f.unique),
                        f.filterable && "Filterable",
                        f.sortable && "Sortable",
                      ]
                        .filter(Boolean)
                        .join(" · ") || "—",
                  },
                ]}
                actions={(f) => (
                  <RowActions
                    title={text(f.label)}
                    edit={() => setEditField({ ...f })}
                    remove={() => remove("fields", text(f.key))}
                  />
                )}
              />
            ) : (
              <EmptyState compact title="No fields yet">
                Add your first field to start defining this entity.
              </EmptyState>
            )}
          </Stack>
        </CardBody>
      </Card>
      <Card>
        <CardBody>
          <Disclosure
            summary="Entity settings"
            meta="Ownership, default fields and sharing"
          >
            <EntitySettings d={d} />
          </Disclosure>
        </CardBody>
      </Card>
    </Stack>
  );
}

function EntitySettings({ d }: { d: Designer }) {
  const { busy, dirty, draft, record, run, save, setDraft, setRecord } = d;
  const fields = items(draft.fields),
    archived = record?.state === "archived";
  return (
    <Stack gap="md">
      <Columns min={200} gap="md">
        <FormField label="Owner field">
          <Select
            value={text(draft.owner_field)}
            onChange={(owner_field) => setDraft({ ...draft, owner_field })}
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
        <p>
          API key <code>{text(draft.key)}</code>
        </p>
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

// The Access tab: permission paths, each one unit of who, which operation,
// which fields and which records. Everything else is denied.
export function AccessTab({ d }: { d: Designer }) {
  const { draft, remove, roles, setEditPermission } = d;
  const fields = items(draft.fields),
    permissions = items(draft.permissions),
    missing = unreached(draft);
  return (
    <Card>
      <CardHeader
        level={3}
        title="Permission paths"
        description="Who reaches which fields, and on which records. Access is denied until a path matches. Administrative roles do not automatically grant data access."
        actions={
          <Button
            variant="primary"
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
      />
      <CardBody>
        <Stack gap="md">
          {permissions.length > 0 && missing.length > 0 && (
            <Alert tone="info" live={false}>
              {missing.map((f) => text(f.label)).join(", ")}{" "}
              {missing.length === 1 ? "is" : "are"} not readable through any
              path yet. New fields never join existing allowlists automatically;
              add them to a path explicitly.
            </Alert>
          )}
          {permissions.length ? (
            <DataTable
              caption="Permission paths"
              rows={permissions}
              rowKey={(p) => text(p.key)}
              columns={[
                {
                  key: "path",
                  header: "Path",
                  render: (p) => <strong>{text(p.label)}</strong>,
                },
                { key: "who", header: "Who", render: (p) => whoOf(p, roles) },
                {
                  key: "can",
                  header: "Can",
                  render: (p) => <Badge mono>{text(p.operation)}</Badge>,
                },
                {
                  key: "fields",
                  header: "Fields",
                  render: (p) =>
                    ["read", "create", "update"].includes(text(p.operation)) ? (
                      strings(p.fields).length ? (
                        <Inline gap="sm">
                          {strings(p.fields).map((key) => (
                            <Badge key={key}>
                              {text(fields.find((f) => f.key === key)?.label) ||
                                key}
                            </Badge>
                          ))}
                        </Inline>
                      ) : (
                        "Record only"
                      )
                    ) : (
                      "Whole record"
                    ),
                },
                {
                  key: "records",
                  header: "Records",
                  render: (p) => (
                    <Stack gap="sm">
                      <span>
                        {p.when
                          ? "Where " + describe(p.when, fields)
                          : "All records"}
                      </span>
                      {!!p.change_when && (
                        <span>
                          Change allowed when {describe(p.change_when, fields)}
                        </span>
                      )}
                    </Stack>
                  ),
                },
              ]}
              actions={(p) => (
                <RowActions
                  title={text(p.label)}
                  edit={() => setEditPermission({ ...p })}
                  remove={() => remove("permissions", text(p.key))}
                />
              )}
            />
          ) : (
            <EmptyState compact title="No access yet">
              Add a Viewer or Editor rule by choosing who, what they can do, and
              which fields.
            </EmptyState>
          )}
        </Stack>
      </CardBody>
    </Card>
  );
}

type RuleRow = {
  key: string;
  title: string;
  kind: string;
  check: string;
  message: string;
  edit: () => void;
  remove?: () => void;
};

// The Rules tab: validation and allowed-change rules, plus fields with
// defaults or calculations.
export function RulesTab({ d }: { d: Designer }) {
  const { draft, remove, setEditField, setEditRule } = d;
  const fields = items(draft.fields);
  const rows: RuleRow[] = [
    ...fields
      .filter((f) => f.default || f.calculation)
      .map((f) => ({
        key: "field:" + text(f.key),
        title: text(f.label),
        kind: f.calculation ? "Calculation" : "Default",
        check: describe(f.calculation || f.default, fields),
        message: "",
        edit: () => setEditField({ ...f }),
      })),
    ...items(draft.rules).map((rule) => ({
      key: "rule:" + text(rule.key),
      title: text(rule.name || rule.key),
      kind: rule.kind === "change" ? "Allowed change" : "Validation",
      check:
        (rule.when ? "When " + describe(rule.when, fields) + ": " : "") +
        describe(rule.assert, fields),
      message: text(rule.message),
      edit: () => setEditRule({ ...rule }),
      remove: () => remove("rules", text(rule.key)),
    })),
  ];
  return (
    <Card>
      <CardHeader
        level={3}
        title="Business rules"
        description="Rules apply to every write, from every App. Set defaults and calculations in a field's editor."
        actions={
          <Button
            variant="primary"
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
      />
      <CardBody>
        {rows.length ? (
          <DataTable
            caption="Business rules"
            rows={rows}
            rowKey={(r) => r.key}
            columns={[
              {
                key: "rule",
                header: "Rule",
                render: (r) => <strong>{r.title}</strong>,
              },
              {
                key: "kind",
                header: "Kind",
                render: (r) => <Badge>{r.kind}</Badge>,
              },
              { key: "check", header: "Checks", render: (r) => r.check },
              {
                key: "message",
                header: "Message",
                render: (r) => r.message || "—",
              },
            ]}
            actions={(r) => (
              <RowActions title={r.title} edit={r.edit} remove={r.remove} />
            )}
          />
        ) : (
          <EmptyState compact title="No rules yet">
            Add validation or allowed-change rules here. Set defaults and
            calculations in a field's editor.
          </EmptyState>
        )}
      </CardBody>
    </Card>
  );
}

// The History tab: the entity's recent publications (the backend returns
// the latest 20), newest first.
export function HistoryTab({ d }: { d: Designer }) {
  const publications = items(d.record?.publications);
  return (
    <Card>
      <CardHeader
        level={3}
        title="Publications"
        description="Each publication is an immutable version. Apps use the active one."
      />
      <CardBody>
        {publications.length ? (
          <DataTable
            caption="Publications"
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
      </CardBody>
    </Card>
  );
}
