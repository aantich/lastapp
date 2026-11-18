import { api } from "./api";
import { refreshQueries } from "./query";
import { useConsoleConfig } from "./console-config";
import { num, type Deployment, type Item, type Release } from "./api-types";
import type { Modal, Step } from "./ui";

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

// What each state means to a person. A state the server adds later still
// shows, under its own name.
const stepCopy: Record<string, [string, string]> = {
  queued: ["Queued", "Release accepted; waiting for earlier deployments"],
  preparing: ["Preparing storage", "Entity storage is prepared"],
  publishing: ["Publishing", "Roles, ACLs and the App switch together"],
  active: ["Active", "Production serves this release"],
};

// A state's short name ("Preparing storage").
export const deploymentStepLabel = (state: string) =>
  stepCopy[state]?.[0] || state.replace(/_/g, " ");

// The Stepper of a running or finished deployment: steps before its state
// are done, its own is active (done once live), later ones pending.
export function deploymentSteps(steps: string[], state: string): Step[] {
  const at = steps.indexOf(state),
    live = at === steps.length - 1;
  return steps.map((id, i) => ({
    id,
    label: stepCopy[id]?.[0] || id.replace(/_/g, " "),
    note: stepCopy[id]?.[1],
    state:
      i < at || (live && i === at) ? "done" : i === at ? "active" : "pending",
  }));
}

// Retrying a failed deployment (Home, the command palette): read its release,
// then confirm a new deployment of it; on success every read refreshes.
export function retryDeployment(
  tenant: string,
  deployment: Deployment,
  setModal: (modal: Modal) => void,
  setError: (message: string) => void,
) {
  void api<Release>("/v1/releases/" + deployment.release_id).then(
    (release) =>
      setModal(deployModal(tenant, release, true, () => refreshQueries())),
    (e: Error) => setError(e.message),
  );
}

// The confirmation that deploys a release to production: a new deployment of
// the same release is also how a failed one is retried (POST
// /v1/releases/{id}/deploy, expecting the production App's revision).
export function deployModal(
  tenant: string,
  release: Release,
  retry: boolean,
  deployed: (result: Deployment) => void,
): Modal {
  return {
    title: (retry ? "Retry deploy of " : "Deploy ") + release.display_name,
    description:
      (retry ? "Starts a new deployment of this release. " : "") +
      "Applies the captured shared Space backend configuration, then activates this App build. Other Apps reuse those Space entities and permissions. Database changes are not automatically rolled back if a step fails.",
    success: "Deployment queued",
    initial: {},
    fields: [],
    submit: retry ? "Retry deploy" : "Deploy to production",
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
