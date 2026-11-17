import { api } from "../api";
import { num, type Item } from "../api-types";
import type { Field } from "../form-modal";
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
export function planForm(p: PlatformScreen, plan?: Item) {
  const { setModal } = p;
  const flags = [
    "shared_database",
    "dedicated_database",
    "managed_database",
    "registered_database",
    "dedicated_compute",
  ];
  const initial: Item = plan
    ? { ...plan }
    : {
        id: crypto.randomUUID(),
        default_layout: "shared",
        allow_shared_database: true,
        allow_managed_database: true,
        allow_registered_database: true,
      };
  flags.forEach((f) => (initial["allow_" + f] = !!initial["allow_" + f]));
  setModal({
    title: plan ? "Edit plan" : "Create plan",
    submit: plan ? "Save" : "Create plan",
    initial,
    fields: [
      { key: "display_name", label: "Name", required: true },
      ...(!plan
        ? [{ key: "code", label: "Code", required: true, advanced: true }]
        : []),
      ...["spaces", "apps", "databases"].map((x) => ({
        key: "max_" + x,
        label: "Maximum " + x,
        type: "number",
        hint: "Blank means unlimited.",
      })),
      {
        key: "default_layout",
        label: "Default database layout",
        type: "select",
        options: [
          { value: "shared", label: "Shared" },
          { value: "dedicated", label: "Dedicated" },
        ],
        required: true,
        advanced: true,
      },
      ...flags.map((f) => ({
        key: "allow_" + f,
        label: "Allow " + f.replaceAll("_", " "),
        type: "checkbox",
        advanced: true,
      })),
    ],
    save: (v) => {
      const out: Item = {
        id: v.id,
        code: v.code,
        display_name: v.display_name,
        default_layout: v.default_layout,
      };
      flags.forEach((f) => (out["allow_" + f] = !!v["allow_" + f]));
      ["spaces", "apps", "databases"].forEach(
        (x) =>
          (out["max_" + x] =
            v["max_" + x] === "" || v["max_" + x] == null
              ? null
              : Number(v["max_" + x])),
      );
      return api(
        "/admin/v1/plans" + (plan ? "/" + plan.id : ""),
        plan ? "PATCH" : "POST",
        out,
        num(plan?.revision),
      );
    },
  });
}
export function serverForm(p: PlatformScreen, server?: Item) {
  const { setModal } = p;
  const fields: Field[] = [
    { key: "display_name", label: "Name", required: true },
    ...(!server
      ? [
          { key: "canonical_host", label: "Hostname / IP", required: true },
          { key: "port", label: "Port", type: "number", required: true },
          {
            key: "tls_mode",
            label: "TLS",
            type: "select",
            options: [
              { value: "verify_identity", label: "Verify server identity" },
              {
                value: "disabled",
                label: "Disabled (local development only)",
              },
            ],
            required: true,
          },
        ]
      : [
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
        ]),
    {
      key: "managed_enabled",
      label: "Allow managed database creation",
      type: "checkbox",
      wide: true,
    },
    {
      key: "username",
      label: "Provisioning username",
      hint: server ? "Leave blank to keep existing credentials." : undefined,
    },
    { key: "password", label: "Provisioning password", type: "password" },
  ];
  setModal({
    title: server ? "Edit database server" : "Add database server",
    submit: server ? "Save" : "Add database server",
    initial: server
      ? { ...server, managed_enabled: !!server.managed_enabled }
      : {
          id: crypto.randomUUID(),
          port: 3306,
          tls_mode: "verify_identity",
          managed_enabled: true,
        },
    fields,
    save: (v) =>
      api(
        "/admin/v1/database-targets" + (server ? "/" + server.id : ""),
        server ? "PATCH" : "POST",
        {
          display_name: v.display_name,
          managed_enabled: !!v.managed_enabled,
          ...(server
            ? { state: v.state }
            : {
                id: v.id,
                canonical_host: v.canonical_host,
                port: Number(v.port),
                tls_mode: v.tls_mode,
              }),
          ...(v.username
            ? { credentials: { username: v.username, password: v.password } }
            : {}),
        },
        num(server?.revision),
      ),
  });
}
