import { api } from "../api";
import { num, type Item } from "../api-types";
import {
  accountFields,
  accountPayload,
  initialPasswordField,
  nameFields,
  stateOptions,
} from "../console-forms";
import type { PlatformScreen } from "./screen";

// The Platform console's forms. Each opens a FormModal through the workspace.
export function addAdmin(p: PlatformScreen) {
  const { tenant, setModal } = p;
  setModal({
    title: "Add administrator",
    submit: "Add administrator",
    description: "Grant administrator access using a new or existing account.",
    initial: { mode: "existing" },
    fields: accountFields(),
    save: (v) =>
      api(
        `/admin/v1/tenants/${tenant}/administrators`,
        "POST",
        accountPayload(v),
      ),
  });
}
export function createTenant(p: PlatformScreen) {
  const { plans, setModal } = p;
  setModal({
    title: "Create Tenant",
    submit: "Create Tenant",
    initial: {
      id: crypto.randomUUID(),
      plan_id: plans[0]?.id,
      admin_mode: "new",
    },
    fields: [
      ...nameFields,
      {
        key: "plan_id",
        label: "Plan",
        type: "select",
        lookup: "/admin/v1/plans",
        options: plans.map((p) => ({ value: p.id, label: p.display_name })),
        required: true,
      },
      ...accountFields("admin_"),
    ],
    save: (v) => {
      const chosen = accountPayload(v, "admin_");
      return api("/admin/v1/tenants", "POST", {
        id: v.id,
        slug: v.slug,
        display_name: v.display_name,
        plan_id: v.plan_id,
        ...(chosen.user_id
          ? { initial_admin_user_id: chosen.user_id }
          : { initial_admin: chosen.account }),
      });
    },
  });
}
export function createUser(p: PlatformScreen) {
  const { setModal } = p;
  setModal({
    title: "Create account",
    submit: "Create account",
    initial: {},
    fields: [
      { key: "display_name", label: "Name", required: true },
      { key: "email", label: "Email", type: "email", required: true },
      initialPasswordField(),
    ],
    save: (v) => api("/admin/v1/users", "POST", v),
  });
}
export function addToTenant(p: PlatformScreen, user: Item) {
  const { setModal } = p;
  setModal({
    title: "Add to Tenant",
    submit: "Add to Tenant",
    success: `${user.display_name} added to the Tenant.`,
    description: `Add ${user.display_name} as an ordinary Tenant member.`,
    initial: {},
    fields: [
      {
        key: "tenant_id",
        label: "Tenant",
        type: "select",
        lookup: "/admin/v1/tenants",
        required: true,
      },
    ],
    save: (v) =>
      api(`/v1/tenants/${v.tenant_id}/memberships`, "POST", {
        user_id: user.id,
      }),
  });
}
export function editTenant(p: PlatformScreen) {
  const { plans, profile, tenant, setModal } = p;
  setModal({
    title: "Edit Tenant",
    submit: "Save",
    initial: {
      ...profile,
      default_layout: profile.default_layout || "shared",
    },
    fields: [
      { key: "display_name", label: "Name", required: true },
      {
        key: "plan_id",
        label: "Plan",
        lookup: "/admin/v1/plans",
        type: "select",
        options: plans.map((p) => ({ value: p.id, label: p.display_name })),
        required: true,
      },
      {
        key: "state",
        label: "Status",
        type: "select",
        options: stateOptions,
        required: true,
      },
      {
        key: "default_layout",
        label: "Default database layout",
        type: "select",
        options: [
          { value: "shared", label: "Shared within Tenant" },
          { value: "dedicated", label: "Dedicated per Space" },
        ],
        advanced: true,
      },
    ],
    save: (v) =>
      api(
        "/admin/v1/tenants/" + tenant,
        "PATCH",
        {
          display_name: v.display_name,
          plan_id: v.plan_id,
          state: v.state,
          default_layout: v.default_layout,
        },
        num(profile.revision),
      ),
  });
}
