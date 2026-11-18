import { useConsoleConfig } from "./console-config";

// The first page of a Space's workspaces that this account owns, as the
// sidebar and Home read it (one cached read for both).
export const workspacesPath = (space: string) =>
  `/v1/spaces/${space}/workspaces?limit=20`;

// What each preparation step means to a person. A step the server adds later
// still shows, under its own name.
const workspaceStepCopy: Record<string, string> = {
  storage: "Preparing its database",
  tables: "Preparing entity tables",
  publishing: "Publishing entities",
};

// A preparing workspace's progress ("Step 2 of 3 · Preparing entity tables"),
// from the steps the server publishes in worker order (GET /v1/config →
// development_policy.workspace_steps) and the step the workspace reached
// (preparation_step). Null when that step is unknown.
export function useWorkspaceProgress() {
  const steps =
    useConsoleConfig().config.development_policy.workspace_steps || [];
  return (step: unknown) => {
    const at = steps.indexOf(String(step));
    if (at < 0) return null;
    const label = workspaceStepCopy[steps[at]] || steps[at].replace(/_/g, " ");
    return {
      value: at + 1,
      max: steps.length,
      label: `Step ${at + 1} of ${steps.length} · ${label}`,
    };
  };
}
