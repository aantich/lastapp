import { useRef } from "react";
import { api, type Failure } from "./api";
import { refreshQueries, useApiQuery } from "./query";
import { errorMessage } from "./use-action";
import { useConsoleConfig } from "./console-config";
import {
  num,
  type Deployment,
  type DeploymentSummary,
  type Item,
  type Release,
} from "./api-types";
import type { Modal, Step } from "./ui";
import { poll } from "./polling";

// A production deployment's progress, derived from the states the server
// publishes (GET /v1/config → development_policy.deployment_states, in worker
// order). The last step is the live state; every earlier one is still running.
export function useDeploymentStates() {
  const { steps, failed } =
    useConsoleConfig().config.development_policy.deployment_states;
  const live = steps[steps.length - 1];
  return {
    steps,
    failed,
    live,
    running: (state: unknown) => steps.slice(0, -1).includes(String(state)),
  };
}

// An App's latest production deployment (GET /v1/apps/{id}/deployment),
// polled while it runs: every 1.5 s, backing off to 10 s after failures, and
// stopping at a final state or when access is gone. refetchInterval never
// overlaps requests. The App header and the Releases tab share this one read.
export function useAppDeployment(app: string, enabled: boolean) {
  const states = useDeploymentStates();
  const failures = useRef({ at: 0, count: 0 });
  const query = useApiQuery<{ deployment?: Deployment | null }>(
    enabled ? `/v1/apps/${app}/deployment` : null,
    {
      refetchInterval: (q) => {
        const { status, error, errorUpdatedAt, data } = q.state;
        if (status === "error") {
          if (failures.current.at !== errorUpdatedAt)
            failures.current = {
              at: errorUpdatedAt,
              count: failures.current.count + 1,
            };
          if ([401, 403, 404].includes((error as Failure).status)) return false;
          return Math.min(
            poll.pending * 2 ** failures.current.count,
            poll.maxBackoff,
          );
        }
        failures.current = { at: 0, count: 0 };
        return states.running(data?.deployment?.state) ? poll.pending : false;
      },
    },
  );
  return {
    // undefined until read (or unreadable), null when there is none.
    deployment: query.data ? query.data.deployment || null : undefined,
    error: query.error
      ? `Could not refresh deployment status: ${errorMessage(query.error)}`
      : "",
  };
}

// What each state means to a person, about the release `name`. A state the
// server adds later still shows, under its own name.
const stepCopy: Record<string, [string, (name: string) => string]> = {
  queued: ["Queued", (name) => `Release ${name} accepted`],
  preparing: ["Preparing storage", () => "Entity versions prepared"],
  publishing: ["Publishing", () => "Switching roles, ACLs and App together"],
  active: ["Active", (name) => `Serving ${name}`],
};

// A state's short name ("Preparing storage").
export const deploymentStepLabel = (state: string) =>
  stepCopy[state]?.[0] || state.replace(/_/g, " ");

// The Stepper of a deployment: steps before its state are done, its own is
// active (done once live), later ones pending. A failed deployment marks the
// step it had reached (failed_state) with its error; one recorded before
// failed steps were kept has no such step and returns no steps.
export function deploymentSteps(
  steps: string[],
  d: Pick<
    DeploymentSummary,
    "state" | "failed_state" | "error_message" | "release_name"
  >,
): Step[] {
  const failed = !steps.includes(d.state);
  const at = steps.indexOf(failed ? d.failed_state || "" : d.state),
    live = !failed && at === steps.length - 1;
  if (at < 0) return [];
  return steps.map((id, i) => ({
    id,
    label: deploymentStepLabel(id),
    note:
      failed && i === at
        ? d.error_message || undefined
        : stepCopy[id]?.[1](d.release_name || "release"),
    state:
      i < at || (live && i === at)
        ? "done"
        : i === at
          ? failed
            ? "failed"
            : "active"
          : "pending",
  }));
}

// The confirmation that retries a failed deployment (App page, Home, the
// command palette): POST /v1/deployments/{id}/retry, expecting the
// deployment's revision, re-runs that same deployment of its release from its
// first step under this account's authority. The server refuses it when
// production changed after the failure. `queued` receives the deployment as
// queued again, so a caller can show it before the next read.
export function retryModal(
  deployment: Deployment,
  queued: (d: Deployment) => void,
): Modal {
  return {
    title: `Retry deploy of ${deployment.release_name || "this release"}`,
    description:
      "Runs this deployment again from its first step, with the same release and the same checks. It is refused if production's configuration changed after it failed. Production keeps serving its current release until this one is active.",
    success: "Deployment queued",
    initial: {},
    fields: [],
    submit: "Retry deploy",
    save: async () => {
      const result = await api<{ state: string; attempts: number }>(
        `/v1/deployments/${deployment.id}/retry`,
        "POST",
        {},
        deployment.revision,
      );
      queued({
        ...deployment,
        state: result.state,
        attempts: result.attempts,
        failed_state: null,
        error_message: null,
        finished_at: null,
      });
    },
  };
}

// Retrying from a place that only lists deployments (Home, the palette):
// every read refreshes once it is queued.
export function retryDeployment(
  deployment: Deployment,
  setModal: (modal: Modal) => void,
) {
  setModal(retryModal(deployment, () => void refreshQueries()));
}

// The confirmation that deploys a release to production (POST
// /v1/releases/{id}/deploy, expecting the production App's revision).
export function deployModal(
  tenant: string,
  release: Release,
  deployed: (result: Deployment) => void,
): Modal {
  return {
    title: "Deploy " + release.display_name,
    description:
      "Applies the captured shared Space backend configuration, then activates this App build. Other Apps reuse those Space entities and permissions. Database changes are not automatically rolled back if a step fails.",
    success: "Deployment queued",
    initial: {},
    fields: [],
    submit: "Deploy to production",
    save: async () => {
      let revision: number | undefined = 1;
      try {
        const target = await api<Item>(
          `/v1/tenants/${tenant}/apps/${release.production_app_id}`,
        );
        revision = num(target.revision);
      } catch (e) {
        if ((e as { status?: number }).status !== 404) throw e;
      }
      deployed(
        await api<Deployment>(
          `/v1/releases/${release.id}/deploy`,
          "POST",
          {},
          revision,
        ),
      );
    },
  };
}
