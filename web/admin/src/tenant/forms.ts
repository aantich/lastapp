import { api } from "../api";
import type { Item, Page } from "../api-types";
import { nameFields } from "../console-forms";
import { errorMessage } from "../use-action";
import { computeField, options, type TenantScreen } from "./screen";

export { addDatabase } from "./database-form";

// The Tenant workspace's create forms. Each opens a FormModal through the
// workspace; lookups that seed a form load first.
// setModal is bound to the route it was read on, so a lookup that finishes
// after the user navigates away opens nothing on the new screen.
export async function createSpace(t: TenantScreen) {
  const { base, me, profile, setModal, setError } = t;
  try {
    const [targets, members] = await Promise.all([
      api<Page>(base + "/database-targets"),
      api<Page>(base + "/memberships"),
    ]);
    setModal({
      title: "Create Space",
      submit: "Create Space",
      description:
        "A Space owns shared business objects, rules and a database.",
      initial: {
        id: crypto.randomUUID(),
        mode: profile.allow_managed_database ? "managed" : "existing",
        layout:
          profile.default_layout ||
          (profile.allow_shared_database ? "shared" : "dedicated"),
        target_id: "",
        initial_manager_user_id: me.id,
      },
      fields: [
        ...nameFields,
        {
          key: "initial_manager_user_id",
          label: "Initial Space manager",
          type: "select",
          advanced: true,
          lookup: base + "/memberships",
          options: options(members.items, "user_id"),
          required: true,
        },
        {
          key: "mode",
          label: "Database",
          advanced: !!profile.allow_managed_database,
          type: "select",
          required: true,
          options: [
            ...(profile.allow_managed_database
              ? [{ value: "managed", label: "Create managed database" }]
              : []),
            { value: "existing", label: "Use an existing database resource" },
          ],
        },
        {
          key: "resource_id",
          label: "Existing database",
          type: "select",
          lookup: base + "/database-resources",
          required: true,
          when: (v) => v.mode === "existing",
        },
        {
          key: "layout",
          label: "New database layout",
          advanced: true,
          type: "select",
          required: true,
          options: [
            ...(profile.allow_shared_database
              ? [{ value: "shared", label: "Shared within this Tenant" }]
              : []),
            ...(profile.allow_dedicated_database
              ? [{ value: "dedicated", label: "Dedicated to this Space" }]
              : []),
          ],
          when: (v) => v.mode === "managed",
        },
        {
          key: "target_id",
          label: "Managed database target",
          advanced: true,
          type: "select",
          lookup: base + "/database-targets?managed=true",
          options: options(
            targets.items.filter((t: Item) => t.managed_enabled),
          ),
          when: (v) => v.mode === "managed",
        },
      ],
      save: (v) =>
        api(base + "/spaces", "POST", {
          id: v.id,
          slug: v.slug,
          display_name: v.display_name,
          initial_manager_user_id: v.initial_manager_user_id,
          database:
            v.mode === "existing"
              ? { resource_id: v.resource_id }
              : {
                  mode: "managed",
                  layout: v.layout,
                  target_id: v.target_id || "",
                },
        }),
    });
  } catch (e) {
    setError(errorMessage(e));
  }
}
export function createApp(t: TenantScreen) {
  const { base, me, profile, route, space, setModal } = t;
  setModal({
    title: "Create App",
    submit: "Create App",
    description:
      route.space === space.id && space.workspace_id
        ? "Develop this App in your isolated workspace. Production capacity is checked when you deploy."
        : "The App reuses its Space’s database and definitions.",
    initial: {
      id: crypto.randomUUID(),
      initial_manager_user_id: me.id,
      compute_mode: "shared",
      space_id: route.space,
    },
    fields: [
      ...nameFields,
      ...(!route.space
        ? [
            {
              key: "space_id",
              label: "Space",
              type: "select",
              lookup: base + "/spaces",
              required: true,
            },
          ]
        : []),
      {
        key: "initial_manager_user_id",
        label: "Initial App manager",
        advanced: true,
        when: () => !!route.space,
        type: "select",
        options: [{ value: me.id, label: me.display_name }],
        lookup: route.space
          ? base + "/spaces/" + route.space + "/recipients"
          : base + "/recipients",
        required: true,
        hint: "An active Tenant member.",
      },
      computeField(!!profile.allow_dedicated_compute),
    ],
    save: (v) =>
      api(base + `/spaces/${v.space_id || route.space}/apps`, "POST", {
        id: v.id,
        slug: v.slug,
        display_name: v.display_name,
        initial_manager_user_id: v.initial_manager_user_id,
        compute_mode: v.compute_mode,
      }),
  });
}
export function createMicro(t: TenantScreen) {
  const { base, route, setModal } = t;
  setModal({
    title: "Add micro-app",
    submit: "Add micro-app",
    description: "Record a client experience within this App.",
    initial: { id: crypto.randomUUID(), navigation_order: 0 },
    fields: [
      ...nameFields,
      { key: "artifact_ref", label: "Artifact reference", advanced: true },
      {
        key: "navigation_order",
        label: "Navigation order",
        type: "number",
        advanced: true,
      },
    ],
    save: (v) =>
      api(base + `/apps/${route.app}/microapps`, "POST", {
        ...v,
        navigation_order: Number(v.navigation_order),
      }),
  });
}
