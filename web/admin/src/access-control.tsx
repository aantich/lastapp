import { useState } from "react";
import { Plus } from "lucide-react";
import { api } from "./api";
import {
  Alert,
  AssignmentList,
  Button,
  EmptyState,
  FormModal,
  Inline,
  Skeleton,
  Stack,
  type Modal,
  Icon,
} from "./ui";
import { useAction } from "./use-action";
import { useApiQuery } from "./query";
import {
  num,
  text,
  type Item,
  type Page,
  type TenantCapabilities,
} from "./api-types";
import {
  assignModal,
  confirmRemoveAccess,
  roleModal,
} from "./access-control/access-modals";
import { scopeAuthority } from "./access-control/scope-authority";
import { ScopeRoles } from "./access-control/scope-roles";
import { SpaceMatrix } from "./access-control/space-matrix";
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
  const { readsSpace, canManage, canDelegate, canEditEntities } =
    scopeAuthority(caps, scopeKind, scopeId, meta);
  const assignmentsQuery = useApiQuery<Page>(
    scopeKind === "space" || metaQuery.data || metaQuery.error
      ? `${path}/role-assignments?${readsSpace ? "include=space&" : ""}page=${page}&limit=${pageSize}`
      : null,
  );
  const { busy, run: act } = useAction(path, {
    onSuccess: () => changed(),
    onError: setError,
  });
  const scopeName = scopeKind === "space" ? "Space" : "App";
  function changed() {
    setModal(null);
  }
  function assign() {
    setModal(assignModal(path, canDelegate));
  }
  function roleForm(role?: Item) {
    setModal(roleModal(path, role));
  }
  async function remove(r: Item) {
    if (await confirmRemoveAccess(r, scopeName))
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
          canEditEntities={canEditEntities}
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
              icon={<Icon icon={Plus} size="sm" aria-hidden />}
              onClick={assign}
            >
              Add access
            </Button>
          )
        }
        items={assignments.map((r) => {
          const inherited = r.scope_kind !== scopeKind;
          return {
            id: text(r.id),
            role: text(r.access_name),
            who: text(r.principal_name),
            scope: `${r.scope_kind === "app" ? "App" : "Space"} · ${text(r.scope_name)}`,
            inherited,
            action: !inherited &&
              canManage &&
              (r.domain !== "control" || canDelegate) && (
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
          };
        })}
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
      <ScopeRoles
        path={path}
        scopeKind={scopeKind}
        scopeName={scopeName}
        canManage={canManage}
        roleForm={roleForm}
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
