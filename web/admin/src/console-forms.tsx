import type { Field } from "./ui";
import { text, type Item } from "./api-types";
export const nameFields: Field[] = [
  { key: "display_name", label: "Name", required: true },
  { key: "slug", label: "Slug", required: true, advanced: true },
];
export const stateOptions = ["active", "suspended", "archived"].map(
  (value) => ({ value, label: value }),
);
export const status = (state: unknown) => (
  <span className={"badge " + String(state)}>
    <i />
    {text(state)}
  </span>
);
export function initialPasswordField(key = "password"): Field {
  return {
    key,
    label: "Initial password",
    type: "password",
    required: true,
    generate: true,
    newPassword: true,
    hint: "Copy it before saving; it must be changed at first login.",
  };
}
export function accountFields(prefix = ""): Field[] {
  return [
    {
      key: prefix + "mode",
      label: "Account",
      wide: true,
      type: "select",
      required: true,
      options: [
        { value: "existing", label: "Use an existing account" },
        { value: "new", label: "Create an email/password account" },
      ],
    },
    {
      key: prefix + "user_id",
      label: "Existing account",
      type: "select",
      lookup: "/admin/v1/users",
      excludePlatform: true,
      excludeInactive: true,
      required: true,
      when: (v) => v[prefix + "mode"] === "existing",
    },
    {
      key: prefix + "name",
      label: "Account name",
      required: true,
      when: (v) => v[prefix + "mode"] === "new",
    },
    {
      key: prefix + "email",
      label: "Email",
      type: "email",
      required: true,
      when: (v) => v[prefix + "mode"] === "new",
    },
    {
      ...initialPasswordField(prefix + "password"),
      when: (v) => v[prefix + "mode"] === "new",
    },
  ];
}
export function accountPayload(v: Item, prefix = "") {
  return v[prefix + "mode"] === "existing"
    ? { user_id: v[prefix + "user_id"] }
    : {
        account: {
          display_name: v[prefix + "name"],
          email: v[prefix + "email"],
          password: v[prefix + "password"],
        },
      };
}
