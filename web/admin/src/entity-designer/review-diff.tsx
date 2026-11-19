import { items, text, type Item } from "../api-types";
import { Badge, DataTable, FactList, Stack } from "../ui";

const kinds: Record<string, string> = {
  fields: "Field",
  permissions: "Access rule",
  rules: "Business rule",
};

type Change = {
  key: string;
  change: string;
  kind: string;
  name: string;
  note: string;
};

// What publishing changes: each added, changed or removed field, access
// rule and business rule, with what to check for it.
export function ReviewDiff({ before, after }: { before?: Item; after: Item }) {
  if (!before)
    return (
      <FactList
        facts={[
          { label: "Change", value: "New entity" },
          {
            label: "Contents",
            value: `${items(after.fields).length} fields · ${items(after.permissions).length} access rules · ${items(after.rules).length} business rules`,
          },
          {
            label: "Access",
            value: "No data is accessible until an access rule grants it.",
          },
        ]}
      />
    );
  const name = (v: Item) => text(v.label || v.name || v.key);
  const changes: Change[] = ["fields", "permissions", "rules"].flatMap((k) => {
    const previous = items(before[k]),
      next = items(after[k]);
    const row = (change: string, v: Item, note: string): Change => ({
      key: change + k + text(v.key),
      change,
      kind: kinds[k],
      name: name(v),
      note,
    });
    return [
      ...next
        .filter((n) => !previous.some((p) => p.key === n.key))
        .map((v) =>
          row("Add", v, k === "fields" ? "Select its access explicitly" : ""),
        ),
      ...next
        .filter((n) =>
          previous.some(
            (p) => p.key === n.key && JSON.stringify(n) !== JSON.stringify(p),
          ),
        )
        .map((v) =>
          row(
            "Change",
            v,
            k === "permissions"
              ? "Review recipients, fields and conditions; existing shares may become inactive"
              : "",
          ),
        ),
      ...previous
        .filter((p) => !next.some((n) => n.key === p.key))
        .map((v) =>
          row(
            "Remove",
            v,
            k === "fields" ? "Unsupported for published fields" : "",
          ),
        ),
    ];
  });
  return (
    <Stack gap="sm">
      {changes.length ? (
        <DataTable
          caption="Changes to publish"
          rows={changes}
          rowKey={(c) => c.key}
          columns={[
            {
              key: "change",
              header: "Change",
              render: (c) => (
                <Badge
                  tone={
                    c.change === "Add"
                      ? "success"
                      : c.change === "Remove"
                        ? "danger"
                        : "info"
                  }
                >
                  {c.change}
                </Badge>
              ),
            },
            { key: "kind", header: "Kind", render: (c) => c.kind },
            {
              key: "name",
              header: "Name",
              render: (c) => <strong>{c.name}</strong>,
            },
            { key: "note", header: "Check", render: (c) => c.note || "—" },
          ]}
        />
      ) : (
        <p>The draft matches the published definition.</p>
      )}
      <p>
        Every inheriting App uses the newly published version. Existing records
        are checked before activation.
      </p>
    </Stack>
  );
}
