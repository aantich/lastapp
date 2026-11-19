import { api } from "../api";
import { num, text, type Item } from "../api-types";
import { confirmAction, type Modal } from "../ui";

const spaceManager = "00000000-0000-4000-8000-000000000005",
  appManager = "00000000-0000-4000-8000-000000000006";

const isManager = (role: unknown) =>
  role === spaceManager || role === appManager;

// Grants one role in a Space or App (at `path`) to a person or a team.
export function assignModal(path: string, canDelegate: boolean): Modal {
  return {
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
  };
}

// Creates a custom role in a Space or App (at `path`), or edits one.
export function roleModal(path: string, role?: Item): Modal {
  return {
    title: role ? "Edit role" : "Create role",
    submit: role ? "Save" : "Create role",
    initial: role ? { ...role } : { id: crypto.randomUUID(), state: "active" },
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
        num(role?.revision),
      ),
  };
}

// Asks before removing one role assignment `r` from this Space or App.
export function confirmRemoveAccess(r: Item, scopeName: string) {
  return confirmAction({
    title: "Remove access?",
    message: `${text(r.principal_name)} loses the ${text(r.access_name)} role in this ${scopeName}. Their other roles stay as they are.`,
    confirmLabel: "Remove access",
    danger: true,
  });
}
