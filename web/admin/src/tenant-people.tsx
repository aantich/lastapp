import { useBuiltinRoles } from "./console-config";
import { useState } from "react";
import { api } from "./api";
import {
  Alert,
  Card,
  confirmAction,
  EmptyState,
  FormModal,
  type Modal,
} from "./ui";
import { useAction } from "./use-action";
import { useApiQuery } from "./query";
import { num, type Item, type TenantCapabilities } from "./api-types";
import {
  addPersonModal,
  addTeamMemberModal,
  createTeamModal,
  editTeamModal,
} from "./tenant-people/forms";
import { TenantMembers } from "./tenant-people/members";
import { TeamList } from "./tenant-people/teams";
import { TeamMembers } from "./tenant-people/team-members";
export function TenantPeople({
  tenant,
  caps,
  tab,
  team,
  selectTeam,
}: {
  tenant: string;
  caps: TenantCapabilities;
  tab: string;
  team: string;
  selectTeam: (id: string) => void;
}) {
  const roles = useBuiltinRoles();
  const base = "/v1/tenants/" + tenant,
    path = base + "/teams/" + team;
  const [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState("");
  const { busy, run: act } = useAction(path, {
    onSuccess: () => {},
    onError: setError,
  });
  const canMembers = !!caps["membership.read"]?.all,
    manageMembers = !!caps["membership.manage"]?.all,
    manageTeam =
      !!caps["team.manage"]?.all ||
      (caps["team.manage"]?.teams || []).includes(team);
  // The parent keys this screen by Tenant, tab and team, so a new selection
  // starts without the previous modal or error.
  const teamQuery = useApiQuery<Item>(team ? path : null),
    group = teamQuery.data || {},
    shownError = error || teamQuery.error?.message || "";
  const addPerson = () => setModal(addPersonModal(base)),
    createTeam = () => setModal(createTeamModal(base)),
    editTeam = () => setModal(editTeamModal(path, group)),
    addTeamPerson = () => setModal(addTeamMemberModal(path, group));
  function toggle(r: Item, kind: "tenant" | "teamAdmin" | "reviewer") {
    const id = kind === "reviewer" ? r.reviewer_id : r.administrator_id,
      revision =
        kind === "reviewer" ? r.reviewer_revision : r.administrator_revision,
      scope = kind === "tenant" ? base : path;
    return act(() =>
      id
        ? api(
            scope + "/role-assignments/" + id,
            "DELETE",
            undefined,
            num(revision),
          )
        : api(scope + "/role-assignments", "POST", {
            id: crypto.randomUUID(),
            user_id: r.user_id,
            role_id: roles[kind],
          }),
    );
  }
  async function removeMember(r: Item) {
    if (
      await confirmAction({
        title: "Remove from team?",
        message:
          "Remove " +
          r.display_name +
          " from this team and revoke their team roles?",
        confirmLabel: "Remove from team",
        danger: true,
      })
    )
      void act(() =>
        api(
          path + "/members/" + r.user_id,
          "DELETE",
          undefined,
          num(group.revision),
        ),
      );
  }
  return (
    <>
      {shownError && <Alert>{shownError}</Alert>}
      {tab === "members" && canMembers ? (
        <TenantMembers
          key={tenant + "members"}
          base={base}
          caps={caps}
          manageMembers={manageMembers}
          busy={busy}
          act={act}
          toggle={toggle}
          addPerson={addPerson}
        />
      ) : !team ? (
        <TeamList
          base={base}
          manageTeams={caps["team.manage"]?.all}
          createTeam={createTeam}
          selectTeam={selectTeam}
        />
      ) : group.id === team ? (
        <TeamMembers
          key={path}
          path={path}
          group={group}
          manageTeam={manageTeam}
          busy={busy}
          act={act}
          toggle={toggle}
          removeMember={removeMember}
          editTeam={editTeam}
          addTeamPerson={addTeamPerson}
          selectTeam={selectTeam}
        />
      ) : (
        <Card>
          <EmptyState compact>Loading team…</EmptyState>
        </Card>
      )}
      {modal && (
        <FormModal
          key={tenant + team + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={() => setModal(null)}
          lookups={{}}
        />
      )}
    </>
  );
}
