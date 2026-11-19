import {
  AccessMatrix,
  EmptyState,
  Select,
  Skeleton,
  StatusBadge,
  type AccessMatrixRow,
} from "../ui";
import { consoleURL, useConsoleURL } from "../console-url";
import { text } from "../api-types";
import { relationshipLabels } from "../entity-designer/shared";
import { useSpaceMatrix } from "./use-space-matrix";

// "Ana Ruiz, Sales" or "Ana Ruiz, Sales +3": who holds a role.
function holders(names: string[]) {
  if (!names.length) return "No one assigned yet";
  const shown = names.slice(0, 2).join(", ");
  return names.length > 2 ? `${shown} +${names.length - 2}` : shown;
}

// One entity's permission paths as roles (and record relationships) ×
// operations. Cells show the published definition; where the saved draft
// differs, the cell is marked. Before a first publication the draft is
// shown and said to be one. A cell opens the entity on its Access tab.
export function SpaceMatrix({
  tenant,
  space,
  path,
  canEditEntities,
}: {
  tenant: string;
  space: string;
  path: string;
  canEditEntities: boolean;
}) {
  const [, navigate] = useConsoleURL();
  const {
    setChosen,
    assignments,
    entities,
    entity,
    published,
    draft,
    shown,
    roles,
    relationships,
    has,
    detail,
    operations,
    loading,
    failure,
    entityError,
    entityName,
  } = useSpaceMatrix(space, path);
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
                  encodeURIComponent(text(entity.id)) +
                  "&tab=access",
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
