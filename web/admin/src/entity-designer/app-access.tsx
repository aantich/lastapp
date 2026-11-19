import { useState } from "react";
import { api } from "../api";
import { apiKey, queryClient, useApiQuery } from "../query";
import { useAction } from "../use-action";
import { useListKeys } from "../list-keys";
import {
  isItem,
  items,
  num,
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
  Stack,
  TextField,
} from "../ui";
import { PermissionGates } from "./permission-gates";
import { inputValue } from "./shared";
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
              {!access.inherit && (
                <PermissionGates
                  access={access}
                  edit={edit}
                  name="ceiling"
                  keys={gateKeys.ceiling}
                  permissions={permissions}
                  roles={roles}
                  catalog={catalog}
                />
              )}
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
              {!!access.restricted && (
                <PermissionGates
                  access={access}
                  edit={edit}
                  name="restrictions"
                  keys={gateKeys.restrictions}
                  permissions={permissions}
                  roles={roles}
                  catalog={catalog}
                />
              )}
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
