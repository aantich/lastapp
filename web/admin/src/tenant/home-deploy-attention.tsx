import type { ReactNode } from "react";
import { AttentionCard, Button, Progress } from "../ui";
import { isItem, text, type Deployment } from "../api-types";
import { deploymentStepLabel, retryDeployment } from "../deployment";
import type { TenantScreen } from "./screen";
import type { HomeReads } from "./home-reads";

// Home's cards for the listed Apps' failed and running deployments.
export function deploymentAttention(
  t: TenantScreen,
  { deployable, deploymentOf, states }: HomeReads,
) {
  const { go, setModal } = t;
  const retry = (d: Deployment) => retryDeployment(d, setModal);

  const attention: ReactNode[] = [];
  for (const app of deployable) {
    const d = deploymentOf.get(text(app.id));
    if (!isItem(d)) continue;
    const name = text(app.display_name),
      release = d.release_name || "A release";
    if (d.state === states.failed)
      attention.push(
        <AttentionCard
          key={"deploy" + d.id}
          state={d.state}
          stateLabel="Deploy failed"
          meta={name}
          title={`${release} failed to deploy`}
          actions={
            <>
              <Button size="sm" variant="primary" onClick={() => retry(d)}>
                Retry deploy
              </Button>
              <Button
                size="sm"
                onClick={() =>
                  go("app", { app: text(app.id), tab: "releases" })
                }
              >
                View timeline
              </Button>
            </>
          }
        >
          {d.error_message || "The deployment did not finish."}
        </AttentionCard>,
      );
    else if (states.running(d.state)) {
      const at = states.steps.indexOf(d.state);
      attention.push(
        <AttentionCard
          key={"deploy" + d.id}
          state={d.state}
          stateLabel="Deploying"
          tone="warning"
          meta={d.deployed_by ? `by ${d.deployed_by}` : name}
          title={`Deploying ${release} to ${name}`}
          extra={
            <Progress
              value={at + 1}
              max={states.steps.length}
              label={`Step ${at + 1} of ${states.steps.length} · ${deploymentStepLabel(d.state)}`}
            />
          }
          actions={
            <Button
              size="sm"
              onClick={() => go("app", { app: text(app.id), tab: "releases" })}
            >
              View timeline
            </Button>
          }
        >
          Production keeps serving its current release until this one is active.
        </AttentionCard>,
      );
    }
  }
  return attention;
}
