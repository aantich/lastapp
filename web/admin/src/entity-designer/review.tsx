import { useState } from "react";
import { api } from "../api";
import { isItem, items, num, text, type Item } from "../api-types";
import {
  Alert,
  Badge,
  Button,
  DataTable,
  Dialog,
  DialogBody,
  FactList,
  Stack,
  Stepper,
} from "../ui";
import { versionLabel, type Designer } from "./shared";
import { publishSteps } from "./pipeline";

// Reviews the draft against the published definition, validates and
// publishes it.
export function ReviewDialog({ d }: { d: Designer }) {
  const { busy, dirty, draft, record, run, save, setRecord, setReview } = d;
  // The draft the last validation ran on and its outcome: a later edit
  // needs a new run. Before one, the saved draft's diagnostics show.
  const [checked, setChecked] = useState<{ draft: string; ok: boolean }>(),
    current = JSON.stringify(draft),
    fresh = checked?.draft === current;
  const diagnostics = items(record?.diagnostics);
  const nextVersion = versionLabel(record?.next_version_number);
  const validation = fresh
    ? checked?.ok
      ? "passed"
      : "failed"
    : diagnostics.length
      ? "failed"
      : "";
  const revealing = items(draft.fields).some(
    (f) =>
      f.calculation &&
      items(draft.permissions).some(
        (p) =>
          p.operation === "read" &&
          Array.isArray(p.fields) &&
          p.fields.includes(f.key),
      ),
  );
  return (
    <Dialog
      title="Review and publish"
      description="Apps inherit the published definition. Your current draft stays separate until publication succeeds."
      onClose={() => setReview(false)}
      closeLabel="Close editor"
      size="lg"
      closeOnBackdrop={false}
      footer={
        <>
          <Button
            disabled={busy}
            onClick={() =>
              void run(async () => {
                const v = await api<Item>(
                  "/v1/entities/" + record!.id + "/validate",
                  "POST",
                  { definition: draft },
                );
                setRecord({ ...record, diagnostics: v.diagnostics });
                setChecked({
                  draft: current,
                  ok: !items(v.diagnostics).length,
                });
              })
            }
          >
            Validate
          </Button>
          <Button
            variant="primary"
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
            {["Publish", nextVersion].filter(Boolean).join(" ")}
          </Button>
        </>
      }
    >
      <DialogBody>
        <Stack gap="md">
          <Stepper
            label="Publication"
            steps={publishSteps(
              { dirty, validation },
              {
                active: versionLabel(record?.active_version_number),
                next: nextVersion,
              },
            )}
          />
          <ReviewDiff
            before={
              isItem(record?.active_definition)
                ? record.active_definition
                : undefined
            }
            after={draft}
          />
          {revealing && (
            <Alert tone="warning" live={false}>
              Calculated fields with view access can reveal information about
              their inputs. Review their access alongside the formula.
            </Alert>
          )}
          {validation === "passed" && (
            <Alert tone="success">Validation passed. Ready to publish.</Alert>
          )}
          {validation === "failed" && (
            <Alert tone="info" live={false}>
              {fresh
                ? "Validation found problems in your latest changes."
                : "The saved draft has diagnostics. Validate checks your latest changes."}
            </Alert>
          )}
          {validation === "failed" &&
            diagnostics.map((x, i) => (
              <Alert as="p" live={false} key={i}>
                {text(x.path)}: {text(x.message)}
              </Alert>
            ))}
        </Stack>
      </DialogBody>
    </Dialog>
  );
}

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
function ReviewDiff({ before, after }: { before?: Item; after: Item }) {
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
