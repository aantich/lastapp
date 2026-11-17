import { useState } from "react";
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
import { Select } from "../select";
import { FieldChoices, Expression } from "./expression";
import { inputValue, isExpression, optionalText, strings } from "./shared";
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
  if (!access)
    return <p>{error || readError?.message || "Loading data access…"}</p>;
  const permissions: (Item & { entity: Item })[] = contracts.flatMap((c) => {
    const entity = c.definition;
    return isItem(entity)
      ? items(entity.permissions).map((p) => ({ ...p, id: p.id, entity }))
      : [];
  });
  function gates(key: "ceiling" | "restrictions") {
    const keys = gateKeys[key],
      list = items(access![key]);
    return (
      <div className="entity-gates">
        {list.map((g, i) => {
          const p = permissions.find((p) => p.id === g.permission);
          return (
            <section className="panel entity-advanced" key={keys.keys[i]}>
              <label className="field">
                <span>Base Space permission</span>
                <Select
                  value={optionalText(g.permission)}
                  onChange={(permission) =>
                    edit({
                      ...access,
                      [key]: list.map((x, j) =>
                        i === j ? { permission, fields: [] } : x,
                      ),
                    })
                  }
                  options={[
                    { value: "", label: "Choose permission" },
                    ...permissions.map((p) => ({
                      value: text(p.id),
                      label: `${p.entity.label} · ${p.label} (${p.operation})`,
                    })),
                  ]}
                />
              </label>
              {p && (
                <FieldChoices
                  fields={items(p.entity.fields).filter(
                    (f) => Array.isArray(p.fields) && p.fields.includes(f.key),
                  )}
                  value={strings(g.fields)}
                  onChange={(v) =>
                    edit({
                      ...access,
                      [key]: list.map((x, j) =>
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
                      [key]: list.map((x, j) =>
                        i === j
                          ? {
                              ...x,
                              when: e.target.checked
                                ? {
                                    op: "is_not_null",
                                    args: [
                                      {
                                        field:
                                          items(p?.entity.fields)[0]?.key || "",
                                      },
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
              {isExpression(g.when) && p && catalog && (
                <Expression
                  fields={items(p.entity.fields)}
                  catalog={catalog}
                  value={g.when}
                  onChange={(v) =>
                    edit({
                      ...access,
                      [key]: list.map((x, j) =>
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
                value={strings(g.roles)}
                onChange={(v) =>
                  edit({
                    ...access,
                    [key]: list.map((x, j) =>
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
                    [key]: list.filter((_, j) => j !== i),
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
              [key]: [...list, { permission: "", fields: [] }],
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
            value={inputValue(access.origin)}
            placeholder="https://crm.example.com"
            onChange={(e) => edit({ ...access, origin: e.target.value })}
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={
              typeof access.inherit === "boolean" ? access.inherit : undefined
            }
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
          checked={
            typeof access.restricted === "boolean"
              ? access.restricted
              : undefined
          }
          onChange={(e) => edit({ ...access, restricted: e.target.checked })}
        />{" "}
        Add App restrictions or App-role requirements
      </label>
      {!!access.restricted && gates("restrictions")}
      <button className="primary" disabled={saving}>
        Save data access
      </button>
    </form>
  );
}
