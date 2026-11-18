import { text, type Item } from "../api-types";
import type { Step } from "../ui";

// The publish pipeline, driven by real state: the saved draft, the last
// validation of this exact draft, and the latest publication's state
// (queued → publishing → prepared/published, or failed). Versions are the
// active and next version labels ("v8", "v9"), when known.
export function publishSteps(
  {
    dirty,
    validation,
    publication,
    changes,
  }: {
    dirty: boolean;
    // The draft's changes to the active version (null: nothing published).
    changes?: number | null;
    // While reviewing: "passed", "failed" or "" (not yet run for this draft).
    validation?: "passed" | "failed" | "";
    // After publishing: the latest publication record.
    publication?: Item;
  },
  versions: { active?: string; next?: string } = {},
): Step[] {
  const state = text(publication?.state);
  const queued = !!publication;
  const step = (
    id: string,
    label: string,
    s: Step["state"],
    note: string,
  ): Step => ({ id, label, state: s, note });
  const count =
    changes === null
      ? "First version"
      : changes
        ? changes + (changes === 1 ? " change" : " changes")
        : "";
  return [
    dirty
      ? step(
          "draft",
          "Drafted",
          "active",
          count ? count + " · not saved" : "Not saved yet",
        )
      : step("draft", "Drafted", "done", count || "Draft saved"),
    queued
      ? step("validate", "Validated", "done", "Checked when queued")
      : validation === "passed"
        ? step("validate", "Validated", "done", "No problems found")
        : validation === "failed"
          ? step("validate", "Validated", "failed", "Problems found")
          : step("validate", "Validated", "pending", "Run Validate"),
    queued
      ? step("review", "Review", "done", "Publication queued")
      : step("review", "Review", "active", "Waiting for you"),
    state === "failed"
      ? step("prepare", "Prepare storage", "failed", "Failed · see below")
      : state === "queued" || state === "publishing"
        ? step(
            "prepare",
            "Prepare storage",
            "active",
            state === "queued" ? "Queued" : "In progress",
          )
        : state === "prepared" || state === "published"
          ? step("prepare", "Prepare storage", "done", "Storage ready")
          : step(
              "prepare",
              "Prepare storage",
              "pending",
              versions.active
                ? "Keep " + versions.active + " live"
                : "Live data unaffected",
            ),
    state === "published"
      ? step("publish", "Published", "done", "Apps use this version")
      : state === "prepared"
        ? step("publish", "Published", "active", "Waits for deployment")
        : step(
            "publish",
            "Published",
            "pending",
            versions.next
              ? "Apps switch to " + versions.next
              : "Apps switch over",
          ),
  ];
}
