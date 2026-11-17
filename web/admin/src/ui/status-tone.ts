export type Tone = "success" | "warning" | "danger" | "info" | "neutral";

// The one state → tone map for every lifecycle state the backend reports
// (registry CHECK constraints in design/control_plane.sql): Tenants, Spaces,
// Apps, micro-apps, users, memberships, roles and permissions (active,
// suspended, archived, disabled), database resources (pending, provisioning,
// ready, failed, retired), entity publications (queued, publishing, prepared,
// published, failed), static releases and deployments (queued, preparing,
// publishing, ready, active, failed, archived) and agent grants/keys
// (active, revoked, rotated). Work in progress is a warning (the prototype's
// "◐ Preparing"); drafts and reviews are info. Unknown states are neutral.
const tones: Record<string, Tone> = {
  active: "success",
  ready: "success",
  live: "success",
  published: "success",
  prepared: "success",
  succeeded: "success",
  pending: "warning",
  queued: "warning",
  provisioning: "warning",
  preparing: "warning",
  publishing: "warning",
  deploying: "warning",
  running: "warning",
  suspended: "warning",
  draft: "info",
  review: "info",
  new: "info",
  failed: "danger",
  error: "danger",
  disabled: "danger",
  revoked: "danger",
  archived: "neutral",
  retired: "neutral",
  rotated: "neutral",
  consumed: "neutral",
  superseded: "neutral",
};

export function statusTone(state: unknown): Tone {
  return tones[String(state ?? "").toLowerCase()] || "neutral";
}
