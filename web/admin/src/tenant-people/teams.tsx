import { Plus } from "lucide-react";
import { Button, CardHeader, Collection, Icon } from "../ui";
import { status } from "../console-forms";
import { text } from "../api-types";

// The Tenant's teams (People › Teams); a row opens its team.
export function TeamList({
  base,
  manageTeams,
  createTeam,
  selectTeam,
}: {
  base: string;
  manageTeams: boolean | undefined;
  createTeam: () => void;
  selectTeam: (id: string) => void;
}) {
  return (
    <Collection
      path={base + "/teams"}
      caption="Teams"
      empty="No teams yet."
      header={
        <CardHeader
          title="Teams"
          description="Teams group Tenant members. A team can receive Space and App business roles; its own Administrator and Reviewer roles apply within the team only."
          actions={
            manageTeams && (
              <Button
                variant="primary"
                icon={<Icon icon={Plus} size="md" aria-hidden />}
                onClick={createTeam}
              >
                Create team
              </Button>
            )
          }
        />
      }
      columns={[
        ["Team", (r) => <strong>{text(r.display_name)}</strong>],
        ["Status", (r) => status(r.state)],
      ]}
      actions={(r) => (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => selectTeam(text(r.id))}
        >
          Open team
        </Button>
      )}
    />
  );
}
