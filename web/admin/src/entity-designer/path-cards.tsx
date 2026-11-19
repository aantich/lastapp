import { Button, Callout, PathCard } from "../ui";
import { items, text, type Item } from "../api-types";
import { RowActions } from "./editor-dialog";
import { describe, strings, whoOf, type Designer } from "./shared";

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

// The draft's permission paths, one PathCard each: who, the operation, the
// record conditions and the fields. With `editable`, each card names its
// rule and offers edit/remove.
export function PathCards({
  d,
  editable,
}: {
  d: Designer;
  editable?: boolean;
}) {
  const { draft, remove, roles, setEditPermission } = d;
  const fields = items(draft.fields);
  const cards = items(draft.permissions).map((p) => {
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
          p.change_when && "change when " + describe(p.change_when, fields),
        ].filter((c): c is string => !!c)}
        fields={
          fieldOperations.includes(text(p.operation))
            ? keys.length
              ? "Fields: " + keys.join(", ")
              : "Fields: none (record only)"
            : "Whole record"
        }
        aside={
          editable && (
            <>
              {text(p.label)}
              <RowActions
                title={text(p.label)}
                edit={() => setEditPermission({ ...p })}
                remove={() => remove("permissions", text(p.key))}
              />
            </>
          )
        }
      />
    );
  });
  return <>{cards}</>;
}

// The fields no read path reaches, with a button adding them to the first
// read path. Nothing when every field is reachable or no path exists.
export function UnreachedFields({ d }: { d: Designer }) {
  const { draft, roles, setDraft } = d;
  const permissions = items(draft.permissions),
    missing = unreached(draft),
    target = permissions.find((p) => p.operation === "read");
  if (!permissions.length || !missing.length) return null;
  return (
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
  );
}
