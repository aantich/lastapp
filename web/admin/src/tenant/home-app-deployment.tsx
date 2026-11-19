import { StatusBadge } from "../ui";
import type { Deployment } from "../api-types";
import type { useDeploymentStates } from "../deployment";
import { ago } from "../actors";

// An App card's status pill, production address and release line, from its
// latest deployment (read only where this account may manage releases).
export function appDeployment(
  appState: string,
  d: Deployment | null | undefined,
  readable: boolean,
  states: ReturnType<typeof useDeploymentStates>,
) {
  if (!readable || d === undefined)
    return { status: <StatusBadge state={appState} /> };
  if (!d)
    return {
      status: <StatusBadge state="draft" tone="neutral" label="Not deployed" />,
      footer: "No production release yet",
    };
  const release = <strong>{d.release_name || "release"}</strong>;
  const address = d.url.replace(/^https?:\/\//, "").replace(/\/$/, "");
  if (d.state === states.live)
    return {
      status: <StatusBadge state={d.state} label="Live" />,
      address,
      footer: (
        <>
          Production {release} · deployed {ago(d.finished_at || d.created_at)}
          {d.deployed_by && ` by ${d.deployed_by}`}
        </>
      ),
    };
  if (d.state === states.failed)
    return {
      status: (
        <StatusBadge
          state={d.state}
          label={`${d.release_name || "Deploy"} failed`}
        />
      ),
      address,
      footer: (
        <>
          {release} failed {ago(d.finished_at || d.created_at)} · production
          unchanged
        </>
      ),
    };
  return {
    status: <StatusBadge state={d.state} tone="warning" label="Deploying" />,
    address,
    footer: (
      <>
        Deploying {release} · started {ago(d.created_at)}
        {d.deployed_by && ` by ${d.deployed_by}`}
      </>
    ),
  };
}
