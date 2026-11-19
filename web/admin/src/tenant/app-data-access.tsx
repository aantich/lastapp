import { AppDataAccess } from "../entity-designer/app-access";
import {
  Badge,
  Card,
  CardRow,
  Disclosure,
  Inline,
  Stack,
  TitledCard,
  ValueBox,
} from "../ui";
import { useApiQuery } from "../query";
import { isItem, items, optText, text, type Item } from "../api-types";
import type { TenantScreen } from "./screen";

// The App's data access: the one approved origin and the ceiling of Space
// permission paths it may use (each with its fields), then the editor.
export function DataAccess({ t }: { t: TenantScreen }) {
  const { app, scoped } = t;
  const id = text(app.id);
  const state = useApiQuery<Item>(`/v1/apps/${id}/data-access`, {
    refetchInterval: false,
  }).data;
  const contracts = useApiQuery<Item>(`/v1/apps/${id}/entity-contracts`, {
    refetchInterval: false,
  }).data;
  const access = isItem(state?.access) ? state.access : undefined;
  const paths = items(contracts?.entities).flatMap((c) =>
    isItem(c.definition)
      ? items(c.definition.permissions).map((path) => ({
          path,
          entity: text((c.definition as Item).label),
        }))
      : [],
  );
  const names = (v: unknown) =>
    Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
  // Each gate keeps its fields: the gate's own narrowing, else every field
  // the Space permission reaches.
  const ceiling = items(access?.ceiling).map((g) => {
    const p = paths.find((x) => x.path.id === g.permission);
    const own = names(g.fields);
    return {
      key: text(g.permission),
      label: p ? `${p.entity} · ${text(p.path.label)}` : text(g.permission),
      fields: own.length ? own : names(p?.path.fields),
    };
  });
  return (
    <Stack>
      <CardRow>
        <TitledCard
          layout="stacked"
          title="Approved origins"
          note="Browsers allowed to sign in to this App’s data API."
        >
          {!access ? null : text(access.origin) ? (
            <ValueBox>{text(access.origin)}</ValueBox>
          ) : (
            <ValueBox>No origin approved yet</ValueBox>
          )}
        </TitledCard>
        <TitledCard
          layout="stacked"
          title="Permission ceiling"
          note="The App can narrow Space access, never widen it."
        >
          {access && (
            <Inline>
              {access.inherit ? (
                <Badge pill>All current and future Space permissions</Badge>
              ) : ceiling.length ? (
                ceiling.map((c) => (
                  <Badge pill key={c.key}>
                    {c.label}
                    {c.fields.length ? ` · ${c.fields.join(", ")}` : ""}
                  </Badge>
                ))
              ) : (
                <Badge pill>No Space permissions</Badge>
              )}
            </Inline>
          )}
        </TitledCard>
      </CardRow>
      {scoped("app.manage") && (
        <Card>
          <Disclosure summary="Edit data access">
            <AppDataAccess
              key={id}
              app={id}
              spaceManager={scoped("entity.manage", optText(app.space_id), "")}
            />
          </Disclosure>
        </Card>
      )}
    </Stack>
  );
}
