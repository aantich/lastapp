import { useState } from "react";
import { Plus } from "lucide-react";
import { api } from "./api";
import {
  AccessMatrix,
  Alert,
  AssignmentList,
  Badge,
  Button,
  CardHeader,
  Collection,
  confirmAction,
  EmptyState,
  FormModal,
  Inline,
  Select,
  Skeleton,
  Stack,
  StatusBadge,
  type AccessMatrixRow,
  type Modal,
} from "./ui";
import { useAction } from "./use-action";
import { useApiQuery } from "./query";
import { status } from "./console-forms";
import { consoleURL, useConsoleURL } from "./console-url";
import {
  items,
  num,
  text,
  type EntityCatalog,
  type Item,
  type Page,
  type TenantCapabilities,
} from "./api-types";
import {
  definitionOf,
  describe,
  relationshipLabels,
  strings,
} from "./entity-designer/shared";
const spaceManager = "00000000-0000-4000-8000-000000000005",
  appManager = "00000000-0000-4000-8000-000000000006";
const pageSize = 50;

// A Space's or App's access: for a Space, who may do what to one entity
// (read from its published permission paths, with draft changes marked) and
// who holds which role; for an App, its assignments and roles. Permission
// paths are edited in the entity's Access tab, never cell by cell here.
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
  const [modal, setModal] = useState<Modal | null>(null),
    [error, setError] = useState(""),
    [page, setPage] = useState(1);
  // An App's Space decides delegated authority; a Space is its own scope.
  const metaQuery = useApiQuery<Item>(scopeKind === "app" ? path : null),
    meta = metaQuery.data || {};
  const assignmentsQuery = useApiQuery<Page>(
    `${path}/role-assignments?page=${page}&limit=${pageSize}`,
  );
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
  const scopeName = scopeKind === "space" ? "Space" : "App";
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
  const assignments = assignmentsQuery.data?.items || [],
    more = !!assignmentsQuery.data?.pagination?.has_more;
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
    <Stack gap="lg">
      {(error || metaQuery.error) && (
        <Alert>{error || metaQuery.error?.message}</Alert>
      )}
      {scopeKind === "space" && (
        <SpaceMatrix
          tenant={tenant}
          space={scopeId}
          path={path}
          assignments={assignments}
          canEditEntities={
            !!caps["entity.manage"]?.all ||
            (caps["entity.manage"]?.spaces || []).includes(scopeId)
          }
        />
      )}
      <AssignmentList
        title="Assignments"
        label={`${scopeName} role assignments`}
        actions={
          canManage && (
            <Button
              size="sm"
              variant="primary"
              icon={<Plus size={14} aria-hidden />}
              onClick={assign}
            >
              Add access
            </Button>
          )
        }
        items={assignments.map((r) => ({
          id: text(r.id),
          role: text(r.access_name),
          who: text(r.principal_name),
          scope: `${r.scope_kind === "app" ? "App" : "Space"} · ${text(r.scope_name)}`,
          action: canManage && (r.domain !== "control" || canDelegate) && (
            <Button
              size="sm"
              variant="danger"
              disabled={busy}
              aria-label={`Remove ${text(r.access_name)} from ${text(r.principal_name)}`}
              onClick={() => void remove(r)}
            >
              Remove
            </Button>
          ),
        }))}
        footer={
          (page > 1 || more) && (
            <Inline>
              <Button
                size="sm"
                disabled={page === 1}
                onClick={() => setPage(page - 1)}
              >
                Previous
              </Button>
              <span>Page {page}</span>
              <Button
                size="sm"
                disabled={!more}
                onClick={() => setPage(page + 1)}
              >
                Next
              </Button>
            </Inline>
          )
        }
      >
        {assignmentsQuery.isPending ? (
          <Stack gap="sm">
            <Skeleton width="60%" />
            <Skeleton width="45%" />
          </Stack>
        ) : assignmentsQuery.error ? (
          <Alert live={false}>{assignmentsQuery.error.message}</Alert>
        ) : assignments.length === 0 ? (
          <EmptyState compact title="No one has a role here yet">
            Each assignment grants one role in this {scopeName} to a person or a
            team.
          </EmptyState>
        ) : undefined}
      </AssignmentList>
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
      {modal && (
        <FormModal
          key={path + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={changed}
          lookups={{}}
        />
      )}
    </Stack>
  );
}

// "Ana Ruiz, Sales" or "Ana Ruiz, Sales +3": who holds a role.
function holders(names: string[]) {
  if (!names.length) return "No one assigned yet";
  const shown = names.slice(0, 2).join(", ");
  return names.length > 2 ? `${shown} +${names.length - 2}` : shown;
}

// One entity's permission paths as roles (and record relationships) ×
// operations. Cells show the published definition; where the saved draft
// differs, the cell is marked. Before a first publication the draft is
// shown and said to be one.
function SpaceMatrix({
  tenant,
  space,
  path,
  assignments,
  canEditEntities,
}: {
  tenant: string;
  space: string;
  path: string;
  assignments: Item[];
  canEditEntities: boolean;
}) {
  const [, navigate] = useConsoleURL();
  const [chosen, setChosen] = useState("");
  const entitiesQuery = useApiQuery<Page>(`/v1/spaces/${space}/entities`),
    rolesQuery = useApiQuery<Page>(`${path}/roles?limit=100`),
    catalogQuery = useApiQuery<EntityCatalog>("/v1/entity-capabilities", {
      refetchInterval: false,
    });
  const entities = entitiesQuery.data?.items || [];
  const entity = entities.find((e) => e.id === chosen) || entities[0];
  const entityQuery = useApiQuery<Item>(
    entity ? `/v1/entities/${text(entity.id)}` : null,
  );
  const record = entityQuery.data?.id === entity?.id ? entityQuery.data : null;
  const published = definitionOf(record?.active_definition),
    draft = definitionOf(record?.draft_json),
    shown = published || draft;
  const paths = (d: Item | null) => items(d?.permissions);
  const fields = items(shown?.fields);
  // The rows: the roles this entity's paths name (published or draft), then
  // the record relationships they use.
  const known = rolesQuery.data?.items || [];
  const roles = [
    ...new Set(
      [...paths(published), ...paths(draft)].flatMap((p) => strings(p.roles)),
    ),
  ].map((id) => known.find((r) => r.id === id) || { id, display_name: id });
  const relationships = [
    ...new Set(
      [...paths(published), ...paths(draft)]
        .filter((p) => !strings(p.roles).length && text(p.relationship))
        .map((p) => text(p.relationship)),
    ),
  ];
  const matches = (row: string) => (p: Item) =>
    row.startsWith("rel:")
      ? !strings(p.roles).length && p.relationship === row.slice(4)
      : strings(p.roles).includes(row);
  const has = (d: Item | null, row: string, op: string) =>
    paths(d).some((p) => p.operation === op && matches(row)(p));
  // The record conditions under which a row's paths apply, in words.
  const conditions = (row: string) => [
    ...new Set(
      paths(shown)
        .filter(matches(row))
        .map((p) => describe(p.when, fields))
        .filter(Boolean),
    ),
  ];
  const detail = (who: string, row: string) => {
    const where = conditions(row);
    return where.length ? `${who} · where ${where.join(" or ")}` : who;
  };
  const rows: AccessMatrixRow[] = [
    ...roles.map((r) => {
      const name = text(r.display_name);
      return {
        id: text(r.id),
        label: name,
        name,
        detail: (
          <>
            {detail(
              holders(
                assignments
                  .filter((a) => a.role_id === r.id)
                  .map((a) => text(a.principal_name)),
              ),
              text(r.id),
            )}
            {r.state !== "active" && (
              <>
                {" "}
                <StatusBadge state={r.state} />
              </>
            )}
          </>
        ),
      };
    }),
    ...relationships.map((rel) => ({
      id: "rel:" + rel,
      label: relationshipLabels[rel] || rel,
      name: relationshipLabels[rel] || rel,
      detail: detail("Relationship, not a role", "rel:" + rel),
    })),
  ];
  const operations = catalogQuery.data?.operations || [];
  const loading =
    entitiesQuery.isPending ||
    rolesQuery.isPending ||
    catalogQuery.isPending ||
    (!!entity && !record && !entityQuery.error);
  // Without the roles or the operations there is no matrix; without an
  // entity (none yet, or not readable) the roles are still listed.
  const failure = rolesQuery.error || catalogQuery.error,
    entityError = entitiesQuery.error || entityQuery.error;
  const entityName = text(entity?.label) || text(entity?.entity_key);
  return (
    <AccessMatrix
      title={entity ? `Role matrix · ${entityName}` : "Role matrix"}
      note={
        entityError
          ? `Entity permissions could not be read: ${entityError.message}`
          : !entity
            ? "No entities in this Space yet"
            : !shown
              ? undefined
              : published
                ? canEditEntities
                  ? "Published access · open a cell to edit its permission path"
                  : "Published access"
                : "Draft · not published yet"
      }
      actions={
        entities.length > 1 && (
          <Select
            aria-label="Entity"
            value={text(entity?.id)}
            onChange={setChosen}
            options={entities.map((e) => ({
              value: text(e.id),
              label: text(e.label) || text(e.entity_key),
            }))}
          />
        )
      }
      label={`Who may do what to ${entityName || "this entity"}`}
      columns={operations.map((op) => ({ id: op, label: op }))}
      rows={rows}
      cell={(row, op) => {
        const on = has(shown, row, op);
        const inDraft = published ? has(draft, row, op) : on;
        return {
          on,
          draft: inDraft === on ? undefined : inDraft ? "added" : "removed",
        };
      }}
      onOpen={
        canEditEntities && entity
          ? () =>
              navigate(
                consoleURL.tenant(tenant, "spaces", space, "entities") +
                  "?entity=" +
                  encodeURIComponent(text(entity.id)),
              )
          : undefined
      }
    >
      {failure ? (
        <EmptyState compact title="Access could not be read">
          {failure.message}
        </EmptyState>
      ) : loading ? (
        <EmptyState compact icon={null}>
          <Skeleton width="60%" />
        </EmptyState>
      ) : !entity ? (
        <EmptyState compact title="No entities in this Space yet">
          Who may read and change records is set per entity, in its Access tab.
        </EmptyState>
      ) : !rows.length ? (
        <EmptyState compact title={`No one may use ${entityName} yet`}>
          Add a permission path in the entity&apos;s Access tab.
        </EmptyState>
      ) : undefined}
    </AccessMatrix>
  );
}
