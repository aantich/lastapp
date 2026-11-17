import { Plus } from "lucide-react";
import { api } from "../api";
import { Select } from "../select";
import { items, num, text, type Item } from "../api-types";
import { confirmAction } from "../confirm";
import { FieldChoices } from "./expression";
import { EntityRow } from "./editor-dialog";
import { inputValue, strings, type Designer } from "./shared";

// The Fields tab: the draft's fields and entity-wide settings.
export function FieldsTab({ d }: { d: Designer }) {
  const {
    busy,
    catalog,
    dirty,
    draft,
    record,
    remove,
    run,
    save,
    setDraft,
    setEditField,
    setRecord,
  } = d;
  return (
    <>
      <p className="muted">Choose the information this entity stores.</p>
      <div className="entity-toolbar entity-fields-toolbar">
        <label className="field">
          <span>Name</span>
          <input
            value={inputValue(draft.label)}
            onChange={(e) => setDraft({ ...draft, label: e.target.value })}
          />
        </label>
        <button
          className="primary"
          onClick={() =>
            setEditField({
              key: "",
              label: "",
              type: catalog.types[0],
              max_length: 200,
            })
          }
        >
          <Plus size={15} /> Add field
        </button>
      </div>
      <div className="entity-rows">
        {items(draft.fields).map((f) => (
          <EntityRow
            key={text(f.key)}
            title={text(f.label)}
            summary={
              <>
                {text(f.type)}
                {f.required ? " · Required" : ""}
                {f.default ? " · Default" : ""}
                {f.calculation ? " · Calculated" : ""}
              </>
            }
            edit={() => setEditField({ ...f })}
            remove={() => remove("fields", text(f.key))}
          />
        ))}
        {!items(draft.fields).length && (
          <p className="entity-empty">
            Add your first field to start defining this entity.
          </p>
        )}
      </div>
      <details className="panel entity-settings">
        <summary>
          <span>Entity settings</span>
          <small>Ownership, default fields and sharing</small>
        </summary>
        <div className="entity-settings-content">
          <div className="entity-form-grid">
            <label className="field">
              <span>Owner field</span>
              <Select
                value={text(draft.owner_field)}
                onChange={(owner_field) => setDraft({ ...draft, owner_field })}
                options={[
                  { value: "", label: "None" },
                  ...items(draft.fields)
                    .filter((f) => f.type === "user")
                    .map((f) => ({
                      value: text(f.key),
                      label: text(f.label),
                    })),
                ]}
              />
            </label>
            <label className="field">
              <span>Owning team field</span>
              <Select
                value={text(draft.team_field)}
                onChange={(team_field) => setDraft({ ...draft, team_field })}
                options={[
                  { value: "", label: "None" },
                  ...items(draft.fields)
                    .filter((f) => f.type === "team")
                    .map((f) => ({
                      value: text(f.key),
                      label: text(f.label),
                    })),
                ]}
              />
            </label>
          </div>
          <section className="entity-settings-group">
            <h3>Default fields</h3>
            <p className="muted">
              Included when an App doesn't select specific fields.
            </p>
            <FieldChoices
              fields={items(draft.fields)}
              value={strings(draft.default_projection)}
              onChange={(v) => setDraft({ ...draft, default_projection: v })}
            />
          </section>
          <section className="entity-settings-group">
            <label>
              <input
                type="checkbox"
                checked={!!draft.sharing}
                onChange={(e) =>
                  setDraft({ ...draft, sharing: e.target.checked })
                }
              />{" "}
              Allow record sharing
            </label>
          </section>
          <div className="entity-settings-footer">
            <p className="muted">
              API key <code>{text(draft.key)}</code>
            </p>
            <button
              className="secondary danger"
              disabled={busy || !!record?.pending_version_id}
              onClick={async () => {
                const archived = record?.state === "archived";
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
                        state:
                          record?.state === "archived" ? "active" : "archived",
                      },
                      num(saved.revision),
                    );
                    setRecord(v);
                  });
              }}
            >
              {record?.state === "archived"
                ? "Restore entity"
                : "Archive entity"}
            </button>
          </div>
        </div>
      </details>
    </>
  );
}
// The Access tab: allow rules; everything else is denied.
export function AccessTab({ d }: { d: Designer }) {
  const { draft, remove, roles, setEditPermission } = d;
  return (
    <>
      <div className="entity-toolbar">
        <p>
          Access is denied until an allow rule matches. Administrative roles do
          not automatically grant data access.
        </p>
        <button
          className="primary"
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
          <Plus size={15} /> Add access rule
        </button>
      </div>
      <div className="entity-rows">
        {items(draft.permissions).map((p) => (
          <EntityRow
            key={text(p.key)}
            title={text(p.label)}
            summary={
              <>
                {text(p.relationship) ||
                  strings(p.roles)
                    .map(
                      (id) =>
                        text(roles.find((r) => r.id === id)?.display_name) ||
                        id,
                    )
                    .join(", ")}{" "}
                · {text(p.operation)} ·{" "}
                {strings(p.fields).join(", ") || "Record"}
              </>
            }
            edit={() => setEditPermission({ ...p })}
            remove={() => remove("permissions", text(p.key))}
          />
        ))}
        {!items(draft.permissions).length && (
          <div className="entity-empty">
            Add a Viewer or Editor rule by choosing who, what they can do, and
            which fields.
          </div>
        )}
      </div>
    </>
  );
}
// The Rules tab: validation and allowed-change rules, plus fields with
// defaults or calculations.
export function RulesTab({ d }: { d: Designer }) {
  const { draft, remove, setEditField, setEditRule } = d;
  return (
    <>
      <div className="entity-toolbar">
        <p>Rules apply to every write, from every App.</p>
        <button
          className="primary"
          onClick={() =>
            setEditRule({
              key: "",
              name: "",
              kind: "validation",
              assert: {
                op: "is_not_null",
                args: [{ field: items(draft.fields)[0]?.key || "" }],
              },
              message: "",
            })
          }
        >
          <Plus size={15} /> Add rule
        </button>
      </div>
      <div className="entity-rows">
        {items(draft.fields)
          .filter((f) => f.default || f.calculation)
          .map((f) => (
            <EntityRow
              key={text(f.key)}
              title={text(f.label)}
              summary={f.calculation ? "Calculation" : "Default"}
              edit={() => setEditField({ ...f })}
            />
          ))}
        {items(draft.rules).map((rule) => (
          <EntityRow
            key={text(rule.key)}
            title={text(rule.name || rule.key)}
            summary={
              <>
                {rule.kind === "change" ? "Allowed change" : "Validation"} ·{" "}
                {text(rule.message)}
              </>
            }
            edit={() => setEditRule({ ...rule })}
            remove={() => remove("rules", text(rule.key))}
          />
        ))}
        {!items(draft.rules).length &&
          !items(draft.fields).some((f) => f.default || f.calculation) && (
            <p className="entity-empty">
              Add validation or allowed-change rules here. Set defaults and
              calculations in a field's editor.
            </p>
          )}
      </div>
    </>
  );
}
