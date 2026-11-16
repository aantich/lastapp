import { useBuiltinRoles } from "./console-config";
import { useEffect, useState } from "react";
import { api } from "./api";
import { Collection } from "./collection";
import { FormModal, type Modal } from "./form-modal";
import { useAction } from "./use-action";
import { confirmAction } from "./confirm";
import { nameFields, status, type Item } from "./console-forms";
export function TenantPeople({
  tenant,
  caps,
  tab,
  team,
  selectTeam,
  refresh,
  reload,
}: {
  tenant: string;
  caps: Item;
  tab: string;
  team: string;
  selectTeam: (id: string) => void;
  refresh: () => void;
  reload: number;
}) {
  const roles = useBuiltinRoles();
  const base = "/v1/tenants/" + tenant,
    path = base + "/teams/" + team;
  const [group, setGroup] = useState<Item>({}),
    [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState("");
  const { busy, run: act } = useAction(path, {
    onSuccess: refresh,
    onError: setError,
  });
  const canMembers = !!caps["membership.read"]?.all,
    manageMembers = !!caps["membership.manage"]?.all,
    manageTeam =
      !!caps["team.manage"]?.all ||
      (caps["team.manage"]?.teams || []).includes(team);
  useEffect(() => {
    setModal(null);
    setError("");
  }, [tenant, tab, team]);
  useEffect(() => {
    if (!team) return;
    let live = true;
    api(path)
      .then((v) => {
        if (live) setGroup(v);
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [tenant, team, reload]);
  const name: [string, (r: Item) => any] = [
    "Person",
    (r) => (
      <>
        <strong>{r.display_name}</strong>
        <small>{r.email}</small>
      </>
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
          captured.revision,
        ),
    });
  }
  function addTeamPerson() {
    const captured = group.revision;
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
        ? api(scope + "/role-assignments/" + id, "DELETE", undefined, revision)
        : api(scope + "/role-assignments", "POST", {
            id: crypto.randomUUID(),
            user_id: r.user_id,
            role_id: roles[kind],
          }),
    );
  }
  return (
    <>
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      {tab === "members" && canMembers ? (
        <>
          <div className="section-actions">
            {manageMembers && (
              <button className="primary" onClick={addPerson}>
                Add person
              </button>
            )}
          </div>
          <Collection
            key={tenant + "members"}
            path={base + "/memberships"}
            reload={reload}
            columns={[
              name,
              [
                "Tenant role",
                (r) => (
                  <label className="role-check">
                    <input
                      type="checkbox"
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
                    Administrator
                  </label>
                ),
              ],
              ["Tenant access", (r) => status(r.state)],
              ["Account", (r) => r.user_state],
            ]}
            actions={
              manageMembers
                ? (r) => (
                    <button
                      className="text-button"
                      disabled={busy}
                      onClick={() =>
                        act(() =>
                          api(
                            base + "/memberships/" + r.user_id,
                            "PATCH",
                            {
                              state:
                                r.state === "active" ? "disabled" : "active",
                            },
                            r.revision,
                          ),
                        )
                      }
                    >
                      {r.state === "active"
                        ? "Disable Tenant access"
                        : "Enable Tenant access"}
                    </button>
                  )
                : undefined
            }
          />
        </>
      ) : !team ? (
        <>
          <div className="section-actions">
            {caps["team.manage"]?.all && (
              <button className="primary" onClick={createTeam}>
                Create team
              </button>
            )}
          </div>
          <Collection
            path={base + "/teams"}
            reload={reload}
            columns={[
              ["Team", (r) => <strong>{r.display_name}</strong>],
              ["Status", (r) => status(r.state)],
            ]}
            actions={(r) => (
              <button className="text-button" onClick={() => selectTeam(r.id)}>
                Open team
              </button>
            )}
          />
        </>
      ) : group.id === team ? (
        <>
          <div className="section-actions">
            <button className="text-button" onClick={() => selectTeam("")}>
              ← All teams
            </button>
            {manageTeam && (
              <>
                <button className="secondary" onClick={editTeam}>
                  Edit team
                </button>
                <button
                  className="primary"
                  disabled={group.state !== "active"}
                  onClick={addTeamPerson}
                >
                  Add team member
                </button>
              </>
            )}
          </div>
          <h2 className="section-title">
            {group.display_name} {status(group.state)}
          </h2>
          <Collection
            key={path}
            path={path + "/members"}
            reload={reload}
            columns={[
              name,
              [
                "Roles",
                (r) => (
                  <span className="role-checks">
                    {(["teamAdmin", "reviewer"] as const).map((kind) => (
                      <label key={kind}>
                        <input
                          type="checkbox"
                          aria-label={
                            (kind === "teamAdmin"
                              ? "Team administrator"
                              : "Reviewer") +
                            " for " +
                            r.display_name
                          }
                          checked={
                            !!r[
                              kind === "teamAdmin"
                                ? "administrator_id"
                                : "reviewer_id"
                            ]
                          }
                          disabled={
                            busy ||
                            !manageTeam ||
                            (!r[
                              kind === "teamAdmin"
                                ? "administrator_id"
                                : "reviewer_id"
                            ] &&
                              (group.state !== "active" ||
                                r.state !== "active" ||
                                r.user_state !== "active" ||
                                r.tenant_state !== "active" ||
                                !!r.platform_admin))
                          }
                          onChange={() => toggle(r, kind)}
                        />
                        {kind === "teamAdmin" ? "Administrator" : "Reviewer"}
                      </label>
                    ))}
                  </span>
                ),
              ],
              ["Status", (r) => status(r.state)],
            ]}
            actions={
              manageTeam
                ? (r) => (
                    <>
                      <button
                        className="text-button"
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
                              group.revision,
                            ),
                          )
                        }
                      >
                        {r.state === "active"
                          ? "Disable team access"
                          : "Enable team access"}
                      </button>
                      <button
                        className="text-button danger"
                        disabled={busy}
                        onClick={async () => {
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
                            act(() =>
                              api(
                                path + "/members/" + r.user_id,
                                "DELETE",
                                undefined,
                                group.revision,
                              ),
                            );
                        }}
                      >
                        Remove from team
                      </button>
                    </>
                  )
                : undefined
            }
          />
        </>
      ) : (
        <p>Loading team…</p>
      )}
      {modal && (
        <FormModal
          key={tenant + team + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={() => {
            setModal(null);
            refresh();
          }}
          lookups={{}}
        />
      )}
    </>
  );
}
