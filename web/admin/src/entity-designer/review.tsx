import { api } from "../api";
import { isItem, items, num, text, type Item } from "../api-types";
import { EditorDialog } from "./editor-dialog";
import type { Designer } from "./shared";
import { Alert } from "../ui";

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
      <ReviewDiff
        before={
          isItem(record?.active_definition)
            ? record.active_definition
            : undefined
        }
        after={draft}
      />
      {items(draft.fields).some(
        (f) =>
          f.calculation &&
          items(draft.permissions).some(
            (p) =>
              p.operation === "read" &&
              Array.isArray(p.fields) &&
              p.fields.includes(f.key),
          ),
      ) && (
        <p className="muted">
          Calculated fields with view access can reveal information about their
          inputs. Review their access alongside the formula.
        </p>
      )}
      {items(record?.diagnostics).length > 0 && (
        <p className="muted">
          Saved draft has diagnostics. Validation below checks your latest
          changes.
        </p>
      )}
      {items(record?.diagnostics).map((d, i) => (
        <Alert as="p" live={false} key={i}>
          {text(d.path)}: {text(d.message)}
        </Alert>
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
              if (!items(v.diagnostics).length)
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
                num(saved.revision),
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
          {items(after.fields).length} fields ·{" "}
          {items(after.permissions).length} access rules ·{" "}
          {items(after.rules).length} business rules
        </p>
        <p>No data is accessible until an access rule grants it.</p>
      </div>
    );
  return (
    <ul className="entity-diff">
      {["fields", "permissions", "rules"].flatMap((k) => {
        const previous = items(before[k]),
          next = items(after[k]);
        return [
          ...next
            .filter((n) => !previous.some((p) => p.key === n.key))
            .map((v) => (
              <li key={k + v.key}>
                Add {k}: {text(v.label || v.name || v.key)}
                {k === "fields" ? " — select access explicitly" : ""}
              </li>
            )),
          ...next
            .filter((n) =>
              previous.some(
                (p) =>
                  p.key === n.key && JSON.stringify(n) !== JSON.stringify(p),
              ),
            )
            .map((v) => (
              <li key={k + v.key}>
                Change {k}: {text(v.label || v.name || v.key)}
                {k === "permissions"
                  ? " — review recipients, fields and conditions; existing shares may become inactive"
                  : ""}
              </li>
            )),
          ...previous
            .filter((p) => !next.some((n) => n.key === p.key))
            .map((v) => (
              <li key={k + v.key}>
                Remove {k}: {text(v.label || v.name || v.key)}
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
