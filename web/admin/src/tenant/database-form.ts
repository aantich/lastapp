import { api } from "../api";
import type { Page } from "../api-types";
import { errorMessage } from "../use-action";
import { options, type TenantScreen } from "./screen";

// The Tenant workspace's Add database form; the database targets that seed it
// load first (see forms.ts).
export async function addDatabase(t: TenantScreen) {
  const { base, profile, setModal, setError } = t;
  try {
    const targets = await api<Page>(base + "/database-targets");
    setModal({
      title: "Add database",
      submit: "Add database",
      initial: {
        id: crypto.randomUUID(),
        management_mode: profile.allow_managed_database
          ? "managed"
          : "registered",
        layout:
          profile.default_layout ||
          (profile.allow_shared_database ? "shared" : "dedicated"),
        target_id: targets.items[0]?.id,
      },
      fields: [
        {
          key: "management_mode",
          label: "Method",
          type: "select",
          required: true,
          options: [
            ...(profile.allow_managed_database
              ? [{ value: "managed", label: "Create managed database" }]
              : []),
            ...(profile.allow_registered_database
              ? [{ value: "registered", label: "Register existing database" }]
              : []),
          ],
        },
        {
          key: "target_id",
          label: "Database target",
          type: "select",
          lookup: base + "/database-targets",
          options: options(targets.items),
          required: true,
        },
        {
          key: "layout",
          label: "Layout",
          type: "select",
          required: true,
          options: [
            ...(profile.allow_shared_database
              ? [{ value: "shared", label: "Shared within Tenant" }]
              : []),
            ...(profile.allow_dedicated_database
              ? [{ value: "dedicated", label: "Dedicated to one Space" }]
              : []),
          ],
        },
        {
          key: "database_name",
          label: "Existing database name",
          required: true,
          when: (v) => v.management_mode === "registered",
        },
        {
          key: "username",
          label: "Existing database username",
          required: true,
          when: (v) => v.management_mode === "registered",
        },
        {
          key: "password",
          label: "Existing database password",
          type: "password",
          required: true,
          when: (v) => v.management_mode === "registered",
        },
      ],
      save: (v) =>
        api(base + "/database-resources", "POST", {
          id: v.id,
          management_mode: v.management_mode,
          layout: v.layout,
          target_id: v.target_id,
          ...(v.management_mode === "registered"
            ? {
                database_name: v.database_name,
                credentials: { username: v.username, password: v.password },
              }
            : {}),
        }),
    });
  } catch (e) {
    setError(errorMessage(e));
  }
}
