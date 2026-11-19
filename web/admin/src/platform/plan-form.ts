import { api } from "../api";
import { num, type Item } from "../api-types";
import type { PlatformScreen } from "./screen";

// The Platform console's create/edit plan form (see forms.ts).
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
