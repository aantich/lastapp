import { api } from "../api";
import type { Item } from "../api-types";
import { EditorDialog } from "./editor-dialog";
import type { Designer } from "./shared";

// Reviews the draft against the published definition, validates and
// publishes it.
export function ReviewDialog({ d }: { d: Designer }) {
  const {
    busy,
    dirty,
    draft,
    record,
    run,
    save,
    setError,
    setRecord,
    setReview,
  } = d;
  return (
    <EditorDialog title="Review and publish" close={() => setReview(false)}>
      <p>
        Apps inherit the published definition. Your current draft stays separate
        until publication succeeds.
      </p>
      <ReviewDiff before={record?.active_definition} after={draft} />
      {draft.fields.some(
        (f: Item) =>
          f.calculation &&
          draft.permissions.some(
            (p: Item) => p.operation === "read" && p.fields?.includes(f.key),
          ),
      ) && (
        <p className="muted">
          Calculated fields with view access can reveal information about their
          inputs. Review their access alongside the formula.
        </p>
      )}
      {record?.diagnostics?.length > 0 && (
        <p className="muted">
          Saved draft has diagnostics. Validation below checks your latest
          changes.
        </p>
      )}
      {record?.diagnostics?.map((d: Item, i: number) => (
        <p className="error" key={i}>
          {d.path}: {d.message}
        </p>
      ))}
      <div className="entity-dialog-actions">
        <button
          className="secondary"
          disabled={busy}
          onClick={() =>
            void run(async () => {
              const v = await api<Item>(
                "/v1/entities/" + record!.id + "/validate",
                "POST",
                { definition: draft },
              );
              setRecord({ ...record, diagnostics: v.diagnostics });
              if (!v.diagnostics.length)
                setError("Validation passed. Ready to publish.");
            })
          }
        >
          Validate
        </button>
        <button
          className="primary"
          disabled={busy || !!record?.pending_version_id}
          onClick={() =>
            void run(async () => {
              const saved = dirty ? await save() : record!;
              await api<Item>(
                "/v1/entities/" + saved.id + "/publications",
                "POST",
                {},
                saved.revision,
              );
              const v = await api<Item>("/v1/entities/" + saved.id);
              setRecord(v);
              setReview(false);
            })
          }
        >
          Publish
        </button>
      </div>
    </EditorDialog>
  );
}
function ReviewDiff({ before, after }: { before?: Item; after: Item }) {
  if (!before)
    return (
      <div className="panel">
        <strong>New entity</strong>
        <p>
          {after.fields.length} fields · {after.permissions.length} access rules
          · {after.rules.length} business rules
        </p>
        <p>No data is accessible until an access rule grants it.</p>
      </div>
    );
  return (
    <ul className="entity-diff">
      {["fields", "permissions", "rules"].flatMap((k) => {
        const previous = before[k] || [],
          next = after[k] || [];
        return [
          ...next
            .filter((n: Item) => !previous.some((p: Item) => p.key === n.key))
            .map((v: Item) => (
              <li key={k + v.key}>
                Add {k}: {v.label || v.name || v.key}
                {k === "fields" ? " — select access explicitly" : ""}
              </li>
            )),
          ...next
            .filter((n: Item) =>
              previous.some(
                (p: Item) =>
                  p.key === n.key && JSON.stringify(n) !== JSON.stringify(p),
              ),
            )
            .map((v: Item) => (
              <li key={k + v.key}>
                Change {k}: {v.label || v.name || v.key}
                {k === "permissions"
                  ? " — review recipients, fields and conditions; existing shares may become inactive"
                  : ""}
              </li>
            )),
          ...previous
            .filter((p: Item) => !next.some((n: Item) => n.key === p.key))
            .map((v: Item) => (
              <li key={k + v.key}>
                Remove {k}: {v.label || v.name || v.key}
                {k === "fields" ? " — unsupported for published fields" : ""}
              </li>
            )),
        ];
      })}
      <li>
        Every inheriting App uses the newly published version. Existing records
        are checked before activation.
      </li>
    </ul>
  );
}
