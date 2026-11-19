import { items as itemsOf, text, type Item } from "../api-types";
import { definitionOf, type Designer } from "./shared";
import { EditorDialog } from "./editor-dialog";
import { FieldEditor } from "./field-editor";
import { PermissionEditor } from "./permission-editor";
import { RuleEditor } from "./rule-editor";

// The open entity's editor dialogs: a field, an access rule or a business
// rule, each replacing its entry in the draft on save.
export function EntityEditors({
  d,
  editField,
  editPermission,
  editRule,
  replace,
}: {
  d: Designer;
  editField: Item | null;
  editPermission: Item | null;
  editRule: Item | null;
  replace: (list: string, key: string, v: Item) => void;
}) {
  const { catalog, roles, space, tenant } = d;
  const { setEditField, setEditPermission, setEditRule } = d;
  const record = d.record!;
  return (
    <>
      {editField && (
        <EditorDialog
          title={editField.key ? "Edit field" : "Add field"}
          close={() => setEditField(null)}
        >
          <FieldEditor
            field={editField}
            fields={itemsOf(d.draft.fields)}
            catalog={catalog}
            space={space}
            published={itemsOf(
              definitionOf(record.active_definition)?.fields,
            ).some((f) => f.key === editField.key)}
            save={(f) => {
              replace("fields", text(editField.key), f);
              setEditField(null);
            }}
          />
        </EditorDialog>
      )}
      {editPermission && (
        <EditorDialog
          title="Access rule"
          description="Who can do what, on which fields and which records."
          close={() => setEditPermission(null)}
        >
          <PermissionEditor
            value={editPermission}
            definition={d.draft}
            roles={roles}
            catalog={catalog}
            tenant={tenant}
            save={(v) => {
              replace("permissions", text(editPermission.key), v);
              setEditPermission(null);
            }}
          />
        </EditorDialog>
      )}
      {editRule && (
        <EditorDialog title="Business rule" close={() => setEditRule(null)}>
          <RuleEditor
            value={editRule}
            fields={itemsOf(d.draft.fields)}
            catalog={catalog}
            save={(v) => {
              replace("rules", text(editRule.key), v);
              setEditRule(null);
            }}
          />
        </EditorDialog>
      )}
    </>
  );
}
