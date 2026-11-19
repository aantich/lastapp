import { Plus } from "lucide-react";
import {
  Badge,
  Button,
  ChipRow,
  DataTable,
  EmptyState,
  Inline,
  Panel,
  StatusBadge,
  Icon,
} from "../ui";
import { items, text } from "../api-types";
import { RowActions } from "./editor-dialog";
import { versionLabel, type Designer } from "./shared";
import { ruleRows } from "./rule-rows";

export { FieldsTab } from "./fields-tab";
export { EntitySettings } from "./entity-settings";
export { AccessTab, PathsSummary } from "./access-tab";

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
          icon={<Icon icon={Plus} size="md" aria-hidden />}
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
                    {versionLabel(p.version_number)}
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
