import { useEffect, useState } from "react";
import { api } from "../api";
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
  const [state, set] = useState<Item | null>(null),
    [access, edit] = useState<Item | null>(null),
    [contracts, setContracts] = useState<Item[]>([]),
    [roles, setRoles] = useState<Item[]>([]),
    [catalog, setCatalog] = useState<EntityCatalog | null>(null),
    [error, setError] = useState(""),
    [saving, setSaving] = useState(false);
  useEffect(() => {
    let live = true;
    Promise.all([
      api<Item>("/v1/apps/" + app + "/data-access"),
      api<Item>("/v1/apps/" + app + "/entity-contracts"),
      api<EntityCatalog>("/v1/entity-capabilities"),
    ])
      .then(([s, c, cat]) => {
        if (live) {
          set(s);
          edit(s.access);
          setContracts(c.entities);
          setRoles(c.roles);
          setCatalog(cat);
        }
      })
      .catch((e) => setError(e.message));
    return () => {
      live = false;
    };
  }, [app]);
  if (!access) return <p>{error || "Loading data access…"}</p>;
  const permissions = contracts.flatMap((c) =>
    (c.definition.permissions || []).map((p: Item) => ({
      ...p,
      id: p.id,
      entity: c.definition,
    })),
  );
  function gates(key: string) {
    return (
      <div className="entity-gates">
        {access![key].map((g: Item, i: number) => {
          const p = permissions.find((p) => p.id === g.permission);
          return (
            <section className="panel entity-advanced" key={i}>
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
                onClick={() =>
                  edit({
                    ...access,
                    [key]: access![key].filter((_: Item, j: number) => j !== i),
                  })
                }
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
        setSaving(true);
        setError("");
        api<Item>(
          "/v1/apps/" + app + "/data-access",
          "PATCH",
          access,
          state!.revision,
        )
          .then((v) => {
            set(v);
            edit(v.access);
          })
          .catch((e) => setError(e.message))
          .finally(() => setSaving(false));
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
