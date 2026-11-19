import { Plus } from "lucide-react";
import { Badge, Button, CardHeader, Collection } from "../ui";
import { status } from "../console-forms";
import { text, type Item } from "../api-types";

// A Space's or App's roles: fixed manager roles and custom business roles.
export function ScopeRoles({
  path,
  scopeKind,
  scopeName,
  canManage,
  roleForm,
}: {
  path: string;
  scopeKind: "space" | "app";
  scopeName: string;
  canManage: boolean;
  roleForm: (role?: Item) => void;
}) {
  const createRole = canManage && (
    <Button
      size="sm"
      icon={<Plus size={14} aria-hidden />}
      onClick={() => roleForm()}
    >
      Create role
    </Button>
  );
  return (
    <Collection
      path={path + "/roles"}
      caption={`${scopeName} roles`}
      empty={`No roles in this ${scopeName} yet.`}
      header={
        <CardHeader
          title="Roles"
          description={
            scopeKind === "space"
              ? "Manager roles are fixed and confer administration. Custom roles describe business relationships; what each may do is set per entity, in its Access tab."
              : "Manager roles are fixed and confer administration. Custom roles describe business relationships; App access narrows what the Space allows, never widens it."
          }
          actions={createRole}
        />
      }
      columns={[
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
      ]}
      actions={
        canManage
          ? (r) =>
              r.domain !== "control" && (
                <Button size="sm" variant="ghost" onClick={() => roleForm(r)}>
                  Edit role
                </Button>
              )
          : undefined
      }
    />
  );
}
