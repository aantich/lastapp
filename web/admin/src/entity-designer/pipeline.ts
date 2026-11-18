import { text, type Item } from "../api-types";
import type { Step } from "../ui";

// The publish pipeline, driven by real state: the saved draft, the last
// validation of this exact draft, and the latest publication's state
// (queued → publishing → prepared/published, or failed).
export function publishSteps({
  dirty,
  validation,
  publication,
}: {
  dirty: boolean;
  // While reviewing: "passed", "failed" or "" (not yet run for this draft).
  validation?: "passed" | "failed" | "";
  // After publishing: the latest publication record.
  publication?: Item;
}): Step[] {
  const state = text(publication?.state);
  const queued = !!publication;
  const step = (
    id: string,
    label: string,
    s: Step["state"],
    note: string,
  ): Step => ({ id, label, state: s, note });
  return [
    dirty
      ? step(
          "draft",
          "Drafted",
          "active",
          "Unsaved changes are saved when you publish",
        )
      : step("draft", "Drafted", "done", "Draft saved"),
    queued
      ? step("validate", "Validated", "done", "Checked when queued")
      : validation === "passed"
        ? step("validate", "Validated", "done", "No problems found")
        : validation === "failed"
          ? step("validate", "Validated", "failed", "Fix the problems below")
          : step("validate", "Validated", "pending", "Run Validate"),
    queued
      ? step("review", "Review", "done", "Publication queued")
      : step("review", "Review", "active", "Waiting for you"),
    state === "failed"
      ? step(
          "prepare",
          "Prepare storage",
          "failed",
          "Failed; see the error below",
        )
      : state === "queued" || state === "publishing"
        ? step(
            "prepare",
            "Prepare storage",
            "active",
            state === "queued"
              ? "Queued; live data keeps working"
              : "Preparing; live data keeps working",
          )
        : state === "prepared" || state === "published"
          ? step("prepare", "Prepare storage", "done", "Storage ready")
          : step(
              "prepare",
              "Prepare storage",
              "pending",
              "Live data keeps working while storage is prepared",
            ),
    state === "published"
      ? step("publish", "Published", "done", "Apps use this version")
      : state === "prepared"
        ? step(
            "publish",
            "Published",
            "active",
            "Activates with its deployment",
          )
        : step(
            "publish",
            "Published",
            "pending",
            "Apps switch to the new version",
          ),
  ];
}
