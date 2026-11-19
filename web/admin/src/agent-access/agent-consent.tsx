import { useState } from "react";
import { api } from "../api";
import {
  Alert,
  Card,
  CardBody,
  FactList,
  FlowSection,
  Inline,
  Skeleton,
  Stack,
  StandalonePage,
} from "../ui";
import { useAction } from "../use-action";
import { agentConfigPath } from "../connect-agent";
import { useApiQuery } from "../query";
import type { AgentConfig, AgentRequest, Me, Redirect } from "../api-types";
import { initialGrant } from "./grant-values";
import { AgentGrantForm } from "./grant-form";

export function AgentConsent({ me }: { me: Me }) {
  const [actionError, setError] = useState("");
  const request = new URLSearchParams(location.search).get("request") || "";
  const infoQuery = useApiQuery<AgentRequest>(
      `/v1/agent-access/requests/${encodeURIComponent(request)}`,
    ),
    configQuery = useApiQuery<AgentConfig>(agentConfigPath);
  const decline = useAction(request, {
    onSuccess: () => {},
    onError: setError,
  });
  const info = infoQuery.data,
    config = configQuery.data,
    error =
      actionError ||
      infoQuery.error?.message ||
      configQuery.error?.message ||
      "";
  function deny() {
    void decline.run(async () => {
      const result = await api<Redirect>(
        `/v1/agent-access/requests/${request}`,
        "POST",
        { deny: true },
      );
      location.assign(result.redirect_url);
    });
  }
  return (
    <StandalonePage width={730}>
      {error && <Alert>{error}</Alert>}
      {info && config ? (
        <AgentGrantForm
          config={config}
          me={me}
          title="Connect an agent"
          intro={
            <FlowSection label="Agent">
              <Inline gap="sm">
                <strong>{info.client_name}</strong>
                <span>Returns to {new URL(info.redirect_uri).host}</span>
              </Inline>
              <FactList
                facts={[
                  { label: "Client ID", value: info.client_id, mono: true },
                ]}
              />
              <Alert tone="warning" live={false}>
                {info.client_name} is the name the client gives itself; it is
                not verified. Check the client ID and return address before
                approving. You act as {me.display_name}.
              </Alert>
            </FlowSection>
          }
          initial={{
            ...initialGrant(config, me),
            permissions: config.default_permissions.filter((code) =>
              info.scopes.split(" ").includes(code),
            ),
          }}
          requested={info.scopes.split(" ")}
          submit={async (values) => {
            const result = await api<Redirect>(
              `/v1/agent-access/requests/${request}`,
              "POST",
              values,
            );
            location.assign(result.redirect_url);
          }}
          cancel={deny}
          label="Approve connection"
        />
      ) : (
        !error && (
          <Card>
            <CardBody>
              <Stack gap="sm">
                <Skeleton width="45%" />
                <Skeleton width="70%" />
                <Skeleton width="60%" />
              </Stack>
            </CardBody>
          </Card>
        )
      )}
    </StandalonePage>
  );
}
