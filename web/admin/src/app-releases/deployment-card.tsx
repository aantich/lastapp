import { ExternalLink, RotateCcw } from "lucide-react";
import { deploymentSteps } from "../deployment";
import { Alert, Button, LinkButton, Stepper, TitledCard, Icon } from "../ui";
import type { Deployment } from "../api-types";
import { time } from "./release-dates";

// The production deployment card: it follows the real deployment states.
export function DeploymentCard({
  deployment,
  stateSteps,
  live,
  failed,
  development,
  servingName,
  busy,
  retry,
}: {
  deployment: Deployment | null | undefined;
  stateSteps: string[];
  live: boolean;
  failed: boolean;
  development: boolean;
  servingName: string | undefined;
  busy: boolean;
  retry: (deployment: Deployment) => void;
}) {
  const name = deployment?.release_name || "This release";
  const note = !deployment
    ? "Not deployed yet. Deploy a release to put this App in production."
    : live
      ? `${name} active since ${time(deployment.finished_at || deployment.created_at)}`
      : failed
        ? servingName && !development
          ? `Production still serves ${servingName}.`
          : "Production is unchanged."
        : servingName && !development
          ? `Production keeps serving ${servingName} until ${name} is active.`
          : `Production keeps serving its current release until ${name} is active.`;
  const steps = deployment ? deploymentSteps(stateSteps, deployment) : [];
  return (
    <TitledCard
      className="deployment-status"
      role="status"
      title="Production deployment"
      note={deployment === undefined ? "Loading…" : note}
      actions={
        deployment &&
        (failed ? (
          <Button
            size="sm"
            variant="primary"
            icon={<Icon icon={RotateCcw} size="sm" aria-hidden />}
            disabled={busy}
            onClick={() => retry(deployment)}
          >
            Retry deploy
          </Button>
        ) : (
          live &&
          development && (
            <LinkButton
              href={deployment.url}
              external
              size="sm"
              trailingIcon={<Icon icon={ExternalLink} size="sm" aria-hidden />}
            >
              Open production App
            </LinkButton>
          )
        ))
      }
    >
      {deployment &&
        (steps.length ? (
          <Stepper label="Deployment progress" steps={steps} min="xs" />
        ) : (
          failed && (
            <Alert as="p" live={false} title="Deployment failed">
              {deployment.error_message}
            </Alert>
          )
        ))}
    </TitledCard>
  );
}
