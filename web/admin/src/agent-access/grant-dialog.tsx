import { api } from "../api";
import { Dialog } from "../ui";
import type { AgentConfig, AgentGrant, Me } from "../api-types";
import { initialGrant } from "./grant-values";
import { AgentGrantForm } from "./grant-form";

export function AgentGrantDialog({
  grant,
  config,
  me,
  close,
  done,
}: {
  grant: AgentGrant;
  config: AgentConfig;
  me: Me;
  close: () => void;
  done: () => void;
}) {
  return (
    <Dialog
      title="Agent access"
      description={grant.client_name}
      titleId="agent-dialog-title"
      closeLabel="Close agent access"
      size="lg"
      closeOnBackdrop={false}
      onClose={close}
    >
      <AgentGrantForm
        config={config}
        me={me}
        initial={initialGrant(config, me, grant)}
        locked
        inDialog
        submit={async (values) => {
          await api(
            `/v1/agent-access/${grant.id}`,
            "PATCH",
            values,
            grant.revision,
          );
          done();
        }}
        cancel={close}
        label="Save access"
      />
    </Dialog>
  );
}
