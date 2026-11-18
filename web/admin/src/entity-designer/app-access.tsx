import { useState } from "react";
import { Plus } from "lucide-react";
import { api } from "../api";
import { apiKey, queryClient, useApiQuery } from "../query";
import { useAction } from "../use-action";
import { useListKeys } from "../list-keys";
import {
  isItem,
  items,
  num,
  text,
  type EntityCatalog,
  type Item,
} from "../api-types";
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  Checkbox,
  EmptyState,
  FieldGroup,
  FormField,
  Inline,
  Select,
  Stack,
  TextField,
} from "../ui";
import { FieldChoices, Expression } from "./expression";
import { inputValue, isExpression, optionalText, strings } from "./shared";
// An App's data access under App → Settings: the Space-approved origin and
// ceiling (inherit everything, or explicit permission gates), and the App's
// own narrowing. Gates only pick from the Space's permission paths and their
// fields, so an App can never widen Space authority.
export function AppDataAccess({
  app,
  spaceManager,
}: {
  app: string;
  spaceManager: boolean;
}) {
  const path = "/v1/apps/" + app + "/data-access";
  // The form edits a local copy; these reads stay still while it is open.
  const stateQuery = useApiQuery<Item>(path, { refetchInterval: false }),
    contractsQuery = useApiQuery<Item>(
      "/v1/apps/" + app + "/entity-contracts",
      { refetchInterval: false },
    ),
    catalogQuery = useApiQuery<EntityCatalog>("/v1/entity-capabilities", {
      refetchInterval: false,
    });
  const [access, edit] = useState<Item | null>(null),
    [error, setError] = useState("");
  const state = stateQuery.data,
    contracts = items(contractsQuery.data?.entities),
    roles = items(contractsQuery.data?.roles),
    catalog = catalogQuery.data || null;
  const ready = !!state && !!contractsQuery.data && !!catalog;
  if (ready && !access) edit(isItem(state.access) ? state.access : null);
  const { busy: saving, run } = useAction(app, {
    onSuccess: () => {},
    onError: setError,
  });
  const readError =
    stateQuery.error || contractsQuery.error || catalogQuery.error;
  // Gates have no id of their own; these keys follow them through removal.
  const gateKeys = {
    ceiling: useListKeys(items(access?.ceiling).length),
    restrictions: useListKeys(items(access?.restrictions).length),
  };
  const header = (
    <CardHeader
      title="App data access"
      description="The Space defines the available authority; this App can only narrow it. New Space fields never join an App's allowlist automatically."
    />
  );
  const shownError = error || readError?.message || "";
  if (!access)
    return (
      <Card>
        {header}
        <CardBody>
          {shownError ? (
            <Alert as="p">{shownError}</Alert>
          ) : (
            <EmptyState compact>Loading data access…</EmptyState>
          )}
        </CardBody>
      </Card>
    );
  const permissions: (Item & { entity: Item })[] = contracts.flatMap((c) => {
    const entity = c.definition;
    return isItem(entity)
      ? items(entity.permissions).map((p) => ({ ...p, id: p.id, entity }))
      : [];
  });
  function gates(key: "ceiling" | "restrictions") {
    const keys = gateKeys[key],
      list = items(access![key]);
    const change = (i: number, v: Item) =>
      edit({ ...access, [key]: list.map((x, j) => (i === j ? v : x)) });
    return (
      <Stack gap="md">
        {list.map((g, i) => {
          const p = permissions.find((p) => p.id === g.permission);
          const fields = items(p?.entity.fields);
          return (
            <FieldGroup
              key={keys.keys[i]}
              legend={"Permission gate " + (i + 1)}
              framed
              actions={
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => {
                    keys.remove(i);
                    edit({
                      ...access,
                      [key]: list.filter((_, j) => j !== i),
                    });
                  }}
                >
                  Remove gate
                </Button>
              }
            >
              <FormField label="Base Space permission">
                <Select
                  value={optionalText(g.permission)}
                  onChange={(permission) =>
                    change(i, { permission, fields: [] })
                  }
                  options={[
                    { value: "", label: "Choose permission" },
                    ...permissions.map((p) => ({
                      value: text(p.id),
                      label: `${p.entity.label} · ${p.label} (${p.operation})`,
                    })),
                  ]}
                />
              </FormField>
              {p && (
                <FieldChoices
                  legend="Fields"
                  description="Only fields the Space permission already reaches."
                  fields={fields.filter(
                    (f) => Array.isArray(p.fields) && p.fields.includes(f.key),
                  )}
                  value={strings(g.fields)}
                  onChange={(v) => change(i, { ...g, fields: v })}
                  empty="This permission reaches the record, not fields."
                />
              )}
              <Checkbox
                label="Limit matching records"
                checked={!!g.when}
                onChange={(on) =>
                  change(i, {
                    ...g,
                    when: on
                      ? {
                          op: "is_not_null",
                          args: [{ field: fields[0]?.key || "" }],
                        }
                      : undefined,
                  })
                }
              />
              {isExpression(g.when) && p && catalog && (
                <Expression
                  fields={fields}
                  catalog={catalog}
                  value={g.when}
                  onChange={(v) => change(i, { ...g, when: v })}
                />
              )}
              <FieldChoices
                legend="Optional App roles"
                fields={roles.map((r) => ({
                  key: r.id,
                  label: r.display_name,
                }))}
                value={strings(g.roles)}
                onChange={(v) => change(i, { ...g, roles: v })}
                empty="This App has no roles yet."
              />
            </FieldGroup>
          );
        })}
        <Inline>
          <Button
            size="sm"
            icon={<Plus size={14} aria-hidden />}
            onClick={() =>
              edit({
                ...access,
                [key]: [...list, { permission: "", fields: [] }],
              })
            }
          >
            Add permission gate
          </Button>
        </Inline>
      </Stack>
    );
  }
  return (
    <Card>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void run(async () => {
            const v = await api<Item>(
              path,
              "PATCH",
              access,
              num(state!.revision),
            );
            queryClient.setQueryData(apiKey(path), v);
            edit(isItem(v.access) ? v.access : null);
          });
        }}
      >
        {header}
        <CardBody>
          <Stack gap="md">
            {error && <Alert as="p">{error}</Alert>}
            <FieldGroup
              legend="Space approval"
              description={
                spaceManager
                  ? "The origin this App runs on and the most it may reach."
                  : "A Space manager approves this origin and ceiling."
              }
              disabled={!spaceManager}
            >
              <TextField
                type="url"
                label="App origin"
                value={inputValue(access.origin)}
                placeholder="https://crm.example.com"
                onChange={(e) => edit({ ...access, origin: e.target.value })}
              />
              <Checkbox
                label="Inherit all current and future Space permissions"
                checked={!!access.inherit}
                onChange={(inherit) => edit({ ...access, inherit })}
              />
              {!access.inherit && gates("ceiling")}
            </FieldGroup>
            <FieldGroup
              legend="App narrowing"
              description="Conditions and App-role requirements on top of the approved ceiling."
            >
              <Checkbox
                label="Add App restrictions or App-role requirements"
                checked={!!access.restricted}
                onChange={(restricted) => edit({ ...access, restricted })}
              />
              {!!access.restricted && gates("restrictions")}
            </FieldGroup>
          </Stack>
        </CardBody>
        <CardFooter>
          <Button type="submit" variant="primary" loading={saving}>
            Save data access
          </Button>
        </CardFooter>
      </form>
    </Card>
  );
}
