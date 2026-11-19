import { isItem, items as itemsOf, text, type Item } from "../api-types";
import { versionLabel } from "./shared";
import { publishSteps } from "./pipeline";

// The open entity's publication state as the header and the pipeline panel
// show it.
export function entityStatus(
  record: Item,
  dirty: boolean,
  changes: number | null,
) {
  const publications = itemsOf(record.publications),
    latest = publications[0],
    pending = !!record.pending_version_id,
    archived = record.state === "archived";
  const activeVersion = versionLabel(record.active_version_number),
    nextVersion = versionLabel(record.next_version_number);
  // The latest publication still describes this draft when its hash is the
  // saved draft's: then its state (pending or failed) is the pipeline's.
  const draftHash = isItem(record.draft_contract)
      ? text(record.draft_contract.definition_hash)
      : "",
    current = !!latest && !dirty && text(latest.definition_hash) === draftHash,
    failed = latest?.state === "failed" && (pending || current);
  const showPipeline = pending || failed || changes !== 0;
  const diagnostics = Array.isArray(record.diagnostics)
    ? record.diagnostics
    : [];
  const steps = publishSteps(
    pending || failed
      ? { dirty: false, publication: latest }
      : {
          dirty,
          validation: dirty ? "" : diagnostics.length ? "failed" : "passed",
          changes,
        },
    { active: activeVersion, next: nextVersion },
  );
  return {
    latest,
    pending,
    archived,
    activeVersion,
    nextVersion,
    failed,
    showPipeline,
    steps,
  };
}

export type EntityStatus = ReturnType<typeof entityStatus>;
