import { api } from "../api";
import { num, type Item } from "../api-types";
import type { Field } from "../ui";
import type { PlatformScreen } from "./screen";

// The Platform console's add/edit database server form (see forms.ts).
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
