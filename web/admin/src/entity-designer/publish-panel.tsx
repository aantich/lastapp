import { text, type Item } from "../api-types";
import { Alert, Panel, Stepper } from "../ui";
import { versionLabel } from "./shared";
import type { EntityStatus } from "./entity-status";

// The publication pipeline under the Fields tab: the pending publication's
// progress, or what publishing the draft would do next.
export function PublishPanel({
  record,
  status,
}: {
  record: Item;
  status: EntityStatus;
}) {
  const { failed, latest, nextVersion, pending, steps } = status;
  return (
    <Panel
      title={
        pending
          ? ["Publishing", versionLabel(record.pending_version_number)]
              .filter(Boolean)
              .join(" ")
          : ["Publish", nextVersion].filter(Boolean).join(" ")
      }
      lead="Live data keeps working while storage is prepared."
    >
      <Stepper label="Publication" steps={steps} />
      {failed && (
        <Alert live={false}>
          Publishing failed: {text(latest.error_message)} Review changes and
          publish again.
        </Alert>
      )}
    </Panel>
  );
}
