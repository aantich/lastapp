import { api } from "../api";
import type { Modal } from "../ui";
import { nameFields } from "../console-forms";
import { num, type Item } from "../api-types";

// The Tenant people screen's forms. `base` is the Tenant's API path, `path`
// the open team's.
export function addPersonModal(base: string): Modal {
  return {
    title: "Add person",
    submit: "Add person",
    description:
      "Enter the email of an existing email/password account. Ask a platform administrator to create a new account. For single sign-on accounts, they can use Users → Add to Tenant.",
    initial: {},
    fields: [{ key: "email", label: "Email", type: "email", required: true }],
    save: (v) =>
      api(base + "/memberships", "POST", { account: { email: v.email } }),
  };
}

export function createTeamModal(base: string): Modal {
  return {
    title: "Create team",
    submit: "Create team",
    initial: { id: crypto.randomUUID() },
    fields: nameFields,
    save: (v) => api(base + "/teams", "POST", v),
  };
}

export function editTeamModal(path: string, group: Item): Modal {
  const captured = { ...group };
  return {
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
  };
}

export function addTeamMemberModal(path: string, group: Item): Modal {
  const captured = num(group.revision);
  return {
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
  };
}
