import { useState } from "react";
import { Plus } from "lucide-react";
import { api } from "./api";
import {
  Alert,
  Avatar,
  Badge,
  Button,
  CardHeader,
  Collection,
  confirmAction,
  FormModal,
  Inline,
  SegmentedControl,
  type Modal,
} from "./ui";
import { useAction } from "./use-action";
import { useApiQuery } from "./query";
import { status } from "./console-forms";
import {
  items,
  num,
  text,
  type Item,
  type TenantCapabilities,
} from "./api-types";
const spaceManager = "00000000-0000-4000-8000-000000000005",
  appManager = "00000000-0000-4000-8000-000000000006";
const views = [
  { value: "assignments", label: "People and teams" },
  { value: "roles", label: "Manage roles" },
];
export function AccessControl({
  tenant,
  caps,
  scopeKind,
  scopeId,
}: {
  tenant: string;
  caps: TenantCapabilities;
  scopeKind: "space" | "app";
  scopeId: string;
}) {
  const path = `/v1/tenants/${tenant}/${scopeKind === "space" ? "spaces" : "apps"}/${scopeId}`;
  const [view, setView] = useState("assignments"),
    [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState("");
  // An App's Space decides delegated authority; a Space is its own scope.
  const metaQuery = useApiQuery<Item>(scopeKind === "app" ? path : null),
    meta = metaQuery.data || {};
  const { busy, run: act } = useAction(path, {
    onSuccess: () => changed(),
    onError: setError,
  });
  const canManage =
    !!caps["authorization.manage"]?.all ||
    (caps["authorization.manage"]?.spaces || []).includes(
      scopeKind === "space" ? scopeId : text(meta.space_id),
    ) ||
    (caps["authorization.manage"]?.apps || []).includes(scopeId);
  const canDelegate =
    !!caps["authorization.manage"]?.all ||
    (scopeKind === "app" &&
      (caps["authorization.manage"]?.spaces || []).includes(
        text(meta.space_id),
      ));
  const isManager = (role: unknown) =>
    role === spaceManager || role === appManager;
  const roles = view === "roles",
    scopeName = scopeKind === "space" ? "Space" : "App";
  function changed() {
    setModal(null);
  }
  function assign() {
    setModal({
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
    });
  }
  function roleForm(role?: Item) {
    setModal({
      title: role ? "Edit role" : "Create role",
      submit: role ? "Save" : "Create role",
      initial: role
        ? { ...role }
        : { id: crypto.randomUUID(), state: "active" },
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
    });
  }
  async function remove(r: Item) {
    if (
      await confirmAction({
        title: "Remove access?",
        message: `${text(r.principal_name)} loses the ${text(r.access_name)} role in this ${scopeName}. Their other roles stay as they are.`,
        confirmLabel: "Remove access",
        danger: true,
      })
    )
      void act(() =>
        api(
          path + "/role-assignments/" + r.id,
          "DELETE",
          undefined,
          num(r.revision),
        ),
      );
  }
  return (
    <>
      {(error || metaQuery.error) && (
        <Alert>{error || metaQuery.error?.message}</Alert>
      )}
      <Collection
        key={path + view}
        path={path + (roles ? "/roles" : "/role-assignments")}
        caption={roles ? `${scopeName} roles` : `${scopeName} role assignments`}
        empty={
          roles
            ? `No roles in this ${scopeName} yet.`
            : `No one has a role in this ${scopeName} yet.`
        }
        header={
          <CardHeader
            title={roles ? "Roles" : "Who has access"}
            description={
              roles
                ? "Manager roles are fixed and confer administration. Custom roles describe business relationships; their entity permissions are set in Entity → Access."
                : scopeKind === "space"
                  ? "Each assignment grants one role in this Space to a person or a team. Tenant administration alone grants no access to business records."
                  : "Each assignment grants one role in this App to a person or a team. App access narrows what the Space allows; it never widens it."
            }
            actions={
              <>
                <SegmentedControl
                  label="Access view"
                  options={views}
                  value={view}
                  onChange={setView}
                />
                {canManage && (
                  <Button
                    variant="primary"
                    icon={<Plus size={15} aria-hidden />}
                    onClick={() => (roles ? roleForm() : assign())}
                  >
                    {roles ? "Create role" : "Add access"}
                  </Button>
                )}
              </>
            }
          />
        }
        columns={
          roles
            ? [
                ["Role", (r) => <strong>{text(r.display_name)}</strong>],
                [
                  "Type",
                  (r) =>
                    r.domain === "control" ? (
                      <Badge tone="info">Manager</Badge>
                    ) : (
                      <Badge>Custom</Badge>
                    ),
                ],
                ["Status", (r) => status(r.state)],
                [
                  "Entity permissions",
                  (r) => {
                    if (r.domain === "control") return "Administrative";
                    const granted = items(r.data_permissions);
                    return granted.length ? (
                      <Inline gap="sm">
                        {granted.map((p) => (
                          <Badge key={text(p.id || p.code)}>
                            {text(p.description || p.code)}
                          </Badge>
                        ))}
                      </Inline>
                    ) : (
                      "Configure in Entity → Access"
                    );
                  },
                ],
              ]
            : [
                [
                  "Person or team",
                  (r) => (
                    <Inline gap="sm">
                      <Avatar name={text(r.principal_name)} size={28} />
                      <span>
                        <strong>{text(r.principal_name)}</strong>{" "}
                        <small>{r.group_id ? "Team" : "Person"}</small>
                      </span>
                    </Inline>
                  ),
                ],
                ["Role", (r) => <Badge>{text(r.access_name)}</Badge>],
              ]
        }
        actions={
          canManage
            ? (r) =>
                roles
                  ? r.domain !== "control" && (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => roleForm(r)}
                      >
                        Edit role
                      </Button>
                    )
                  : (r.domain !== "control" || canDelegate) && (
                      <Button
                        size="sm"
                        variant="danger"
                        disabled={busy}
                        onClick={() => void remove(r)}
                      >
                        Remove access
                      </Button>
                    )
            : undefined
        }
      />
      {modal && (
        <FormModal
          key={path + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={changed}
          lookups={{}}
        />
      )}
    </>
  );
}
