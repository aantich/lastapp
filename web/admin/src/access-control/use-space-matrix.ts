import { useState } from "react";
import { useAllPages, useApiQuery } from "../query";
import {
  items,
  text,
  type EntityCatalog,
  type Item,
  type Page,
} from "../api-types";
import { definitionOf, describe, strings } from "../entity-designer/shared";

// The reads behind a Space's role matrix and the permission paths of the
// chosen entity (the first one until another is chosen).
export function useSpaceMatrix(space: string, path: string) {
  const [chosen, setChosen] = useState("");
  const entitiesQuery = useApiQuery<Page>(`/v1/spaces/${space}/entities`),
    rolesQuery = useApiQuery<Page>(`${path}/roles?limit=100`),
    catalogQuery = useApiQuery<EntityCatalog>("/v1/entity-capabilities", {
      refetchInterval: false,
    }),
    // Who holds each role: every Space assignment, not the list's page.
    holdersQuery = useAllPages(`${path}/role-assignments`);
  const assignments = holdersQuery.data?.items || [];
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
  const operations = catalogQuery.data?.operations || [];
  const loading =
    entitiesQuery.isPending ||
    rolesQuery.isPending ||
    holdersQuery.isPending ||
    catalogQuery.isPending ||
    (!!entity && !record && !entityQuery.error);
  // Without the roles or the operations there is no matrix; without an
  // entity (none yet, or not readable) the roles are still listed.
  const failure = rolesQuery.error || catalogQuery.error || holdersQuery.error,
    entityError = entitiesQuery.error || entityQuery.error;
  const entityName = text(entity?.label) || text(entity?.entity_key);
  return {
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
  };
}
