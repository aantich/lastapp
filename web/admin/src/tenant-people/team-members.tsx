import { ArrowLeft, Plus } from "lucide-react";
import { api } from "../api";
import { Button, CardHeader, Checkbox, Collection, Inline, Icon } from "../ui";
import { status } from "../console-forms";
import { num, text, type Item } from "../api-types";
import { personColumn } from "./person-column";

// One team's members (People › Teams › team) and their fixed team roles.
// `path` is the team's API path and `group` the loaded team.
export function TeamMembers({
  path,
  group,
  manageTeam,
  busy,
  act,
  toggle,
  removeMember,
  editTeam,
  addTeamPerson,
  selectTeam,
}: {
  path: string;
  group: Item;
  manageTeam: boolean;
  busy: boolean;
  act: (fn: () => Promise<unknown>) => Promise<void>;
  toggle: (r: Item, kind: "teamAdmin" | "reviewer") => Promise<void>;
  removeMember: (r: Item) => Promise<void>;
  editTeam: () => void;
  addTeamPerson: () => void;
  selectTeam: (id: string) => void;
}) {
  return (
    <Collection
      path={path + "/members"}
      caption={`${text(group.display_name)} members`}
      empty="This team has no members yet."
      header={
        <CardHeader
          title={
            <>
              {text(group.display_name)} {status(group.state)}
            </>
          }
          description="Team members and their fixed team roles. Removing someone from the team also removes their team roles."
          actions={
            <>
              <Button
                variant="ghost"
                icon={<Icon icon={ArrowLeft} size="md" aria-hidden />}
                onClick={() => selectTeam("")}
              >
                All teams
              </Button>
              {manageTeam && (
                <>
                  <Button onClick={editTeam}>Edit team</Button>
                  <Button
                    variant="primary"
                    icon={<Icon icon={Plus} size="md" aria-hidden />}
                    disabled={group.state !== "active"}
                    onClick={addTeamPerson}
                  >
                    Add team member
                  </Button>
                </>
              )}
            </>
          }
        />
      }
      columns={[
        personColumn,
        [
          "Team roles",
          (r) => (
            <Inline gap="md">
              {(["teamAdmin", "reviewer"] as const).map((kind) => {
                const held =
                  !!r[
                    kind === "teamAdmin" ? "administrator_id" : "reviewer_id"
                  ];
                return (
                  <Checkbox
                    key={kind}
                    label={kind === "teamAdmin" ? "Administrator" : "Reviewer"}
                    aria-label={
                      (kind === "teamAdmin"
                        ? "Team administrator"
                        : "Reviewer") +
                      " for " +
                      r.display_name
                    }
                    checked={held}
                    disabled={
                      busy ||
                      !manageTeam ||
                      (!held &&
                        (group.state !== "active" ||
                          r.state !== "active" ||
                          r.user_state !== "active" ||
                          r.tenant_state !== "active" ||
                          !!r.platform_admin))
                    }
                    onChange={() => toggle(r, kind)}
                  />
                );
              })}
            </Inline>
          ),
        ],
        ["Status", (r) => status(r.state)],
      ]}
      actions={
        manageTeam
          ? (r) => (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={busy}
                  onClick={() =>
                    act(() =>
                      api(
                        path + "/members/" + r.user_id,
                        "PATCH",
                        {
                          state: r.state === "active" ? "disabled" : "active",
                        },
                        num(group.revision),
                      ),
                    )
                  }
                >
                  {r.state === "active"
                    ? "Disable team access"
                    : "Enable team access"}
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  disabled={busy}
                  onClick={() => void removeMember(r)}
                >
                  Remove from team
                </Button>
              </>
            )
          : undefined
      }
    />
  );
}
