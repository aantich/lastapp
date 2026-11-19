import { api } from "./api";
import type { Modal } from "./ui";

// The form that creates a development workspace in `space`.
export const createWorkspaceModal = (space: string): Modal => ({
  title: "Create development workspace",
  description:
    "Copies the published Space configuration into a new, isolated database. Records, role assignments and approved App origins are excluded.",
  success: "Workspace created; preparing its database",
  fields: [
    {
      key: "display_name",
      label: "Workspace name",
      required: true,
    },
  ],
  initial: { display_name: "My workspace" },
  submit: "Create workspace",
  save: (v) =>
    api(`/v1/spaces/${space}/workspaces`, "POST", {
      id: crypto.randomUUID(),
      display_name: v.display_name,
    }),
});
