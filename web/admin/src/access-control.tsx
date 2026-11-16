import { useState } from "react";
import { api } from "./api";
import { Collection } from "./collection";
import { FormModal, type Modal } from "./form-modal";
import { useAction } from "./use-action";
import { useApiQuery } from "./query";
import { status } from "./console-forms";
import type { Item } from "./api-types";
const spaceManager = "00000000-0000-4000-8000-000000000005",
  appManager = "00000000-0000-4000-8000-000000000006";
export function AccessControl({
  tenant,
  caps,
  scopeKind,
  scopeId,
}: {
  tenant: string;
  caps: Item;
  scopeKind: "space" | "app";
  scopeId: string;
}) {
  const path = `/v1/tenants/${tenant}/${scopeKind === "space" ? "spaces" : "apps"}/${scopeId}`;
  const [view, setView] = useState("assignments"),
    [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState("");
  // An App's Space decides delegated authority; a Space is its own scope.
  const metaQuery = useApiQuery<Item>(scopeKind === "app" ? path : null),
    meta = metaQuery.data || {};
  const { busy, run: act } = useAction(path, {
    onSuccess: () => changed(),
    onError: setError,
  });
  const canManage =
    !!caps["authorization.manage"]?.all ||
    (caps["authorization.manage"]?.spaces || []).includes(
      scopeKind === "space" ? scopeId : meta.space_id,
    ) ||
    (caps["authorization.manage"]?.apps || []).includes(scopeId);
  const canDelegate =
    !!caps["authorization.manage"]?.all ||
    (scopeKind === "app" &&
      (caps["authorization.manage"]?.spaces || []).includes(meta.space_id));
  const isManager = (role: string) =>
    role === spaceManager || role === appManager;
  function changed() {
    setModal(null);
  }
  function assign() {
    setModal({
      title: "Add access",
      submit: "Add access",
      initial: { id: crypto.randomUUID(), type: "user" },
      fields: [
        {
          key: "role_id",
          label: "Role",
          type: "select",
          lookup: path + "/roles",
          excludeControl: !canDelegate,
          required: true,
        },
        {
          key: "type",
          label: "Recipient",
          type: "select",
          options: [
            { value: "user", label: "Person" },
            { value: "team", label: "Team" },
          ],
          required: true,
          when: (v) => !isManager(v.role_id),
        },
        {
          key: "user_id",
          label: "Person",
          type: "select",
          lookup: path + "/recipients",
          required: true,
          when: (v) => v.type !== "team" || isManager(v.role_id),
        },
        {
          key: "group_id",
          label: "Team",
          type: "select",
          lookup: path + "/recipients?type=team",
          required: true,
          when: (v) => v.type === "team" && !isManager(v.role_id),
        },
      ],
      save: (v) =>
        api(path + "/role-assignments", "POST", {
          id: v.id,
          role_id: v.role_id,
          ...(v.type === "team" && !isManager(v.role_id)
            ? { group_id: v.group_id }
            : { user_id: v.user_id }),
        }),
    });
  }
  function roleForm(role?: Item) {
    setModal({
      title: role ? "Edit role" : "Create role",
      submit: role ? "Save" : "Create role",
      initial: role
        ? { ...role }
        : { id: crypto.randomUUID(), state: "active" },
      fields: [
        { key: "display_name", label: "Name", required: true },
        ...(!role
          ? [{ key: "code", label: "Code", required: true, advanced: true }]
          : []),
        ...(role
          ? [
              {
                key: "state",
                label: "Status",
                type: "select",
                options: ["active", "disabled"].map((value) => ({
                  value,
                  label: value,
                })),
                required: true,
              },
            ]
          : []),
      ],
      save: (v) =>
        api(
          path + "/roles" + (role ? "/" + role.id : ""),
          role ? "PATCH" : "POST",
          role
            ? { display_name: v.display_name, state: v.state }
            : {
                id: v.id,
                code: v.code,
                display_name: v.display_name,
                state: v.state,
              },
          role?.revision,
        ),
    });
  }
  return (
    <>
      <div className="section-actions">
        <button
          className={view === "assignments" ? "secondary" : "text-button"}
          onClick={() => setView("assignments")}
        >
          People and teams
        </button>
        <button
          className={view === "roles" ? "secondary" : "text-button"}
          onClick={() => setView("roles")}
        >
          Manage roles
        </button>
        {canManage && (
          <button
            className="primary"
            onClick={() => (view === "roles" ? roleForm() : assign())}
          >
            {view === "roles" ? "Create role" : "Add access"}
          </button>
        )}
      </div>
      {(error || metaQuery.error) && (
        <div className="error" role="alert">
          {error || metaQuery.error?.message}
        </div>
      )}
      <Collection
        key={path + view}
        path={path + (view === "roles" ? "/roles" : "/role-assignments")}
        columns={
          view === "roles"
            ? [
                ["Role", (r) => <strong>{r.display_name}</strong>],
                [
                  "Type",
                  (r) => (r.domain === "control" ? "Manager" : "Custom"),
                ],
                ["Status", (r) => status(r.state)],
                [
                  "Entity permissions",
                  (r) =>
                    r.domain === "control"
                      ? "Administrative"
                      : (r.data_permissions || [])
                          .map((p: Item) => p.description || p.code)
                          .join(", ") || "Configure in Entity → Access",
                ],
              ]
            : [
                [
                  "Person or team",
                  (r) => (
                    <>
                      <strong>{r.principal_name}</strong>
                      <small>{r.group_id ? "Team" : "Person"}</small>
                    </>
                  ),
                ],
                ["Role", (r) => r.access_name],
              ]
        }
        actions={
          canManage
            ? (r) =>
                view === "roles"
                  ? r.domain !== "control" && (
                      <button
                        className="text-button"
                        onClick={() => roleForm(r)}
                      >
                        Edit role
                      </button>
                    )
                  : (r.domain !== "control" || canDelegate) && (
                      <button
                        className="text-button danger"
                        disabled={busy}
                        onClick={() =>
                          act(() =>
                            api(
                              path + "/role-assignments/" + r.id,
                              "DELETE",
                              undefined,
                              r.revision,
                            ),
                          )
                        }
                      >
                        Remove access
                      </button>
                    )
            : undefined
        }
      />
      {modal && (
        <FormModal
          key={path + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={changed}
          lookups={{}}
        />
      )}
    </>
  );
}
