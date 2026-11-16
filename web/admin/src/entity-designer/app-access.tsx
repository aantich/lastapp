import { useState } from "react";
import { api } from "../api";
import { apiKey, queryClient, useApiQuery } from "../query";
import { useAction } from "../use-action";
import { useListKeys } from "../list-keys";
import type { EntityCatalog, Item } from "../api-types";
import { Select } from "../select";
import { FieldChoices, Expression } from "./expression";
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
    contracts: Item[] = contractsQuery.data?.entities || [],
    roles: Item[] = contractsQuery.data?.roles || [],
    catalog = catalogQuery.data || null;
  const ready = !!state && !!contractsQuery.data && !!catalog;
  if (ready && !access) edit(state.access);
  const { busy: saving, run } = useAction(app, {
    onSuccess: () => {},
    onError: setError,
  });
  const readError =
    stateQuery.error || contractsQuery.error || catalogQuery.error;
  // Gates have no id of their own; these keys follow them through removal.
  const gateKeys = {
    ceiling: useListKeys(access?.ceiling?.length || 0),
    restrictions: useListKeys(access?.restrictions?.length || 0),
  };
  if (!access)
    return <p>{error || readError?.message || "Loading data access…"}</p>;
  const permissions = contracts.flatMap((c) =>
    (c.definition.permissions || []).map((p: Item) => ({
      ...p,
      id: p.id,
      entity: c.definition,
    })),
  );
  function gates(key: "ceiling" | "restrictions") {
    const keys = gateKeys[key];
    return (
      <div className="entity-gates">
        {access![key].map((g: Item, i: number) => {
          const p = permissions.find((p) => p.id === g.permission);
          return (
            <section className="panel entity-advanced" key={keys.keys[i]}>
              <label className="field">
                <span>Base Space permission</span>
                <Select
                  value={g.permission}
                  onChange={(permission) =>
                    edit({
                      ...access,
                      [key]: access![key].map((x: Item, j: number) =>
                        i === j ? { permission, fields: [] } : x,
                      ),
                    })
                  }
                  options={[
                    { value: "", label: "Choose permission" },
                    ...permissions.map((p) => ({
                      value: p.id,
                      label: `${p.entity.label} · ${p.label} (${p.operation})`,
                    })),
                  ]}
                />
              </label>
              {p && (
                <FieldChoices
                  fields={p.entity.fields.filter((f: Item) =>
                    p.fields?.includes(f.key),
                  )}
                  value={g.fields || []}
                  onChange={(v) =>
                    edit({
                      ...access,
                      [key]: access![key].map((x: Item, j: number) =>
                        i === j ? { ...x, fields: v } : x,
                      ),
                    })
                  }
                />
              )}
              <label>
                <input
                  type="checkbox"
                  checked={!!g.when}
                  onChange={(e) =>
                    edit({
                      ...access,
                      [key]: access![key].map((x: Item, j: number) =>
                        i === j
                          ? {
                              ...x,
                              when: e.target.checked
                                ? {
                                    op: "is_not_null",
                                    args: [
                                      { field: p?.entity.fields[0]?.key || "" },
                                    ],
                                  }
                                : undefined,
                            }
                          : x,
                      ),
                    })
                  }
                />{" "}
                Limit matching records
              </label>
              {g.when && p && catalog && (
                <Expression
                  fields={p.entity.fields}
                  catalog={catalog}
                  value={g.when}
                  onChange={(v) =>
                    edit({
                      ...access,
                      [key]: access![key].map((x: Item, j: number) =>
                        i === j ? { ...x, when: v } : x,
                      ),
                    })
                  }
                />
              )}
              <p>Optional App roles</p>
              <FieldChoices
                fields={roles.map((r) => ({
                  key: r.id,
                  label: r.display_name,
                }))}
                value={g.roles || []}
                onChange={(v) =>
                  edit({
                    ...access,
                    [key]: access![key].map((x: Item, j: number) =>
                      i === j ? { ...x, roles: v } : x,
                    ),
                  })
                }
              />
              <button
                type="button"
                className="text-button danger"
                onClick={() => {
                  keys.remove(i);
                  edit({
                    ...access,
                    [key]: access![key].filter((_: Item, j: number) => j !== i),
                  });
                }}
              >
                Remove gate
              </button>
            </section>
          );
        })}
        <button
          type="button"
          className="secondary"
          onClick={() =>
            edit({
              ...access,
              [key]: [...access![key], { permission: "", fields: [] }],
            })
          }
        >
          Add permission gate
        </button>
      </div>
    );
  }
  return (
    <form
      className="panel entity-app-access entity-advanced"
      onSubmit={(e) => {
        e.preventDefault();
        void run(async () => {
          const v = await api<Item>(path, "PATCH", access, state!.revision);
          queryClient.setQueryData(apiKey(path), v);
          edit(v.access);
        });
      }}
    >
      <h2>App data access</h2>
      <p>The Space defines available authority. App settings can narrow it.</p>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <fieldset disabled={!spaceManager}>
        <legend>Space approval</legend>
        <label className="field">
          <span>App origin</span>
          <input
            type="url"
            value={access.origin}
            placeholder="https://crm.example.com"
            onChange={(e) => edit({ ...access, origin: e.target.value })}
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={access.inherit}
            onChange={(e) => edit({ ...access, inherit: e.target.checked })}
          />{" "}
          Inherit all current and future Space permissions
        </label>
        {!access.inherit && gates("ceiling")}
      </fieldset>
      {!spaceManager && (
        <p className="muted">
          A Space manager approves this origin and ceiling.
        </p>
      )}
      <label>
        <input
          type="checkbox"
          checked={access.restricted}
          onChange={(e) => edit({ ...access, restricted: e.target.checked })}
        />{" "}
        Add App restrictions or App-role requirements
      </label>
      {access.restricted && gates("restrictions")}
      <button className="primary" disabled={saving}>
        Save data access
      </button>
    </form>
  );
}
