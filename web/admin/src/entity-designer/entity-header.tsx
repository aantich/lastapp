import { text } from "../api-types";
import { Badge, Button, RecordHeader, StatusBadge } from "../ui";
import type { Designer } from "./shared";
import type { EntityStatus } from "./entity-status";

const plural = (n: number, word: string) =>
  n + " " + word + (n === 1 ? "" : "s");

// The open entity's header: its name, key and published version, its draft
// state, and the save, test and publish actions.
export function EntityHeader({
  d,
  status,
  setTab,
}: {
  d: Designer;
  status: EntityStatus;
  setTab: (tab: string) => void;
}) {
  const { busy, changes, dirty, draft, run, save, setReview } = d;
  const record = d.record!;
  const { activeVersion, archived, pending } = status;
  const label = text(draft.label) || text(record.label);
  const meta = [
    text(draft.key),
    record.active_version_id
      ? ["published", activeVersion].filter(Boolean).join(" ")
      : "not published",
  ].join(" · ");
  return (
    <RecordHeader
      initial={label.slice(0, 1).toUpperCase()}
      title={label}
      meta={meta}
      chips={
        pending ? (
          <StatusBadge state="publishing" />
        ) : archived ? (
          <StatusBadge state="archived" />
        ) : changes === null ? (
          <Badge pill tone="info">
            Draft
          </Badge>
        ) : changes > 0 ? (
          <Badge pill tone="info">
            Draft · {plural(changes, "change")}
          </Badge>
        ) : undefined
      }
      actions={
        <>
          <Button onClick={() => setTab("test")}>Test as user…</Button>
          {dirty && (
            <Button
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  await save();
                })
              }
            >
              Save draft
            </Button>
          )}
          <Button
            variant="primary"
            disabled={busy || pending || archived}
            onClick={() => setReview(true)}
          >
            Review &amp; publish
          </Button>
        </>
      }
    />
  );
}
