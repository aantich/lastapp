import type { ReactNode } from "react";
import { StatusBadge, statusTone, type Tone } from "../ui";
import { optText, text, type DeploymentSummary, type Item } from "../api-types";
import type { useDeploymentStates } from "../deployment";

type States = ReturnType<typeof useDeploymentStates>;

// An App's status pill and the tone of its tile, the one derivation the Apps
// list, the App header and Home share. A disabled or archived App shows its
// own state. Otherwise the latest deployment decides (failed, deploying or
// live) where the reader may see it; without it, the App is live when it
// serves an active release and "Not deployed" when it has none.
export function appStatus(
  app: Item,
  deployment: DeploymentSummary | null | undefined,
  states: States,
): { tone: Tone; badge: ReactNode } {
  const state = text(app.state);
  if (state && state !== "active")
    return { tone: statusTone(state), badge: <StatusBadge state={state} /> };
  if (deployment?.state === states.failed)
    return {
      tone: "danger",
      badge: (
        <StatusBadge
          state={deployment.state}
          label={`${deployment.release_name || "Deploy"} failed`}
        />
      ),
    };
  if (deployment && states.running(deployment.state))
    return {
      tone: "warning",
      badge: (
        <StatusBadge
          state={deployment.state}
          tone="warning"
          label="Deploying"
        />
      ),
    };
  if (deployment?.state === states.live || optText(app.active_release_id))
    return {
      tone: "success",
      badge: <StatusBadge state={states.live} label="Live" />,
    };
  return {
    tone: "neutral",
    badge: (
      <StatusBadge state="not_deployed" tone="neutral" label="Not deployed" />
    ),
  };
}

// An App's hosted address without its scheme ("crm.apps.example.com").
export const hostOf = (origin: unknown) =>
  text(origin)
    .replace(/^https?:\/\//, "")
    .replace(/\/$/, "");
