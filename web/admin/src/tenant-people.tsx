import { useBuiltinRoles } from "./console-config";
import { useState, type ReactNode } from "react";
import { ArrowLeft, Plus } from "lucide-react";
import { api } from "./api";
import {
  Alert,
  Avatar,
  Button,
  Card,
  CardHeader,
  Checkbox,
  Collection,
  confirmAction,
  EmptyState,
  FormModal,
  Inline,
  type Modal,
} from "./ui";
import { useAction } from "./use-action";
import { useApiQuery } from "./query";
import { nameFields, status } from "./console-forms";
import { num, text, type Item, type TenantCapabilities } from "./api-types";
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
  const name: [string, (r: Item) => ReactNode] = [
    "Person",
    (r) => (
      <Inline gap="sm">
        <Avatar name={text(r.display_name)} size={28} />
        <span>
          <strong>{text(r.display_name)}</strong> <small>{text(r.email)}</small>
        </span>
      </Inline>
    ),
  ];
  function addPerson() {
    setModal({
      title: "Add person",
      submit: "Add person",
      description:
        "Enter the email of an existing email/password account. Ask a platform administrator to create a new account. For single sign-on accounts, they can use Users → Add to Tenant.",
      initial: {},
      fields: [{ key: "email", label: "Email", type: "email", required: true }],
      save: (v) =>
        api(base + "/memberships", "POST", { account: { email: v.email } }),
    });
  }
  function createTeam() {
    setModal({
      title: "Create team",
      submit: "Create team",
      initial: { id: crypto.randomUUID() },
      fields: nameFields,
      save: (v) => api(base + "/teams", "POST", v),
    });
  }
  function editTeam() {
    const captured = { ...group };
    setModal({
      title: "Edit team",
      submit: "Save",
      initial: captured,
      fields: [
        { key: "display_name", label: "Name", required: true },
        {
          key: "state",
          label: "Status",
          type: "select",
          required: true,
          options: ["active", "disabled"].map((value) => ({
            value,
            label: value,
          })),
        },
      ],
      save: (v) =>
        api(
          path,
          "PATCH",
          { display_name: v.display_name, state: v.state },
          num(captured.revision),
        ),
    });
  }
  function addTeamPerson() {
    const captured = num(group.revision);
    setModal({
      title: "Add team member",
      submit: "Add team member",
      initial: {},
      fields: [
        {
          key: "user_id",
          label: "Person",
          type: "select",
          lookup: path + "/recipients",
          required: true,
        },
      ],
      save: (v) =>
        api(path + "/members", "POST", { user_id: v.user_id }, captured),
    });
  }
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
        <Collection
          key={tenant + "members"}
          path={base + "/memberships"}
          caption="Tenant members"
          empty="No one is a member of this Tenant yet."
          header={
            <CardHeader
              title="People"
              description="Members of this Tenant. A Tenant administrator assigns Tenant and team roles and Space/App managers; the role grants no access to business records."
              actions={
                manageMembers && (
                  <Button
                    variant="primary"
                    icon={<Plus size={15} aria-hidden />}
                    onClick={addPerson}
                  >
                    Add person
                  </Button>
                )
              }
            />
          }
          columns={[
            name,
            [
              "Tenant role",
              (r) => (
                <Checkbox
                  label="Administrator"
                  aria-label={"Tenant administrator for " + r.display_name}
                  checked={!!r.administrator_id}
                  disabled={
                    busy ||
                    !caps["authorization.manage"]?.all ||
                    (!r.administrator_id &&
                      (r.state !== "active" ||
                        r.user_state !== "active" ||
                        !!r.platform_admin))
                  }
                  onChange={() => toggle(r, "tenant")}
                />
              ),
            ],
            ["Tenant access", (r) => status(r.state)],
            ["Account", (r) => status(r.user_state)],
          ]}
          actions={
            manageMembers
              ? (r) => (
                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={busy}
                    onClick={() =>
                      act(() =>
                        api(
                          base + "/memberships/" + r.user_id,
                          "PATCH",
                          {
                            state: r.state === "active" ? "disabled" : "active",
                          },
                          num(r.revision),
                        ),
                      )
                    }
                  >
                    {r.state === "active"
                      ? "Disable Tenant access"
                      : "Enable Tenant access"}
                  </Button>
                )
              : undefined
          }
        />
      ) : !team ? (
        <Collection
          path={base + "/teams"}
          caption="Teams"
          empty="No teams yet."
          header={
            <CardHeader
              title="Teams"
              description="Teams group Tenant members. A team can receive Space and App business roles; its own Administrator and Reviewer roles apply within the team only."
              actions={
                caps["team.manage"]?.all && (
                  <Button
                    variant="primary"
                    icon={<Plus size={15} aria-hidden />}
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
      ) : group.id === team ? (
        <Collection
          key={path}
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
                    icon={<ArrowLeft size={15} aria-hidden />}
                    onClick={() => selectTeam("")}
                  >
                    All teams
                  </Button>
                  {manageTeam && (
                    <>
                      <Button onClick={editTeam}>Edit team</Button>
                      <Button
                        variant="primary"
                        icon={<Plus size={15} aria-hidden />}
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
            name,
            [
              "Team roles",
              (r) => (
                <Inline gap="md">
                  {(["teamAdmin", "reviewer"] as const).map((kind) => {
                    const held =
                      !!r[
                        kind === "teamAdmin"
                          ? "administrator_id"
                          : "reviewer_id"
                      ];
                    return (
                      <Checkbox
                        key={kind}
                        label={
                          kind === "teamAdmin" ? "Administrator" : "Reviewer"
                        }
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
                              state:
                                r.state === "active" ? "disabled" : "active",
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
