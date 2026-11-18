import { useConsoleConfig } from "./console-config";
import type { Step } from "./ui";

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
