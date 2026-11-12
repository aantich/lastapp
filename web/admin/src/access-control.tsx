import { Search } from "lucide-react";
import { useEffect, useState } from "react";
import { api } from "./api";
import { RemoteSelect } from "./remote-select";

type Item = Record<string, any>;
type Kind = "platform" | "tenant" | "team" | "space" | "app";
const scopes: [Kind, string][] = [
  ["tenant", "Tenant"],
  ["team", "Teams"],
  ["space", "Spaces"],
  ["app", "Apps"],
];

export function AccessControl({
  tenant,
  platform,
  caps,
  refresh,
  scopeKind,
  scopeId,
}: {
  tenant: string;
  platform: boolean;
  caps: Item;
  refresh: () => void;
  scopeKind?: Kind;
  scopeId?: string;
}) {
  const defaultKind = (): Kind =>
    scopeKind ||
    (!tenant
      ? "platform"
      : platform || caps["authorization.read"]?.all
        ? "tenant"
        : caps["space.read"]?.available
          ? "space"
          : caps["app.read"]?.available
            ? "app"
            : "team");
  const [kind, setKind] = useState<Kind>(defaultKind);
  const [target, setTarget] = useState(scopeId || "");
  const [view, setView] = useState("assignments");
  const [records, setRecords] = useState<Item[]>([]),
    [roles, setRoles] = useState<Item[]>([]),
    [members, setMembers] = useState<Item[]>([]);
  const [memberPage, setMemberPage] = useState(1),
    [memberQuery, setMemberQuery] = useState(""),
    [memberMore, setMemberMore] = useState(false);
  const [resource, setResource] = useState<Item>({}),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0),
    [page, setPage] = useState(1),
    [more, setMore] = useState(false),
    [query, setQuery] = useState("");
  const [form, setForm] = useState(""),
    [editing, setEditing] = useState<Item | null>(null),
    [values, setValues] = useState<Item>({});
  const base = `/v1/tenants/${tenant}`;
  const path =
    kind === "platform"
      ? "/admin/v1"
      : kind === "tenant"
        ? base
        : `${base}/${kind === "team" ? "teams" : kind === "space" ? "spaces" : "apps"}/${target}`;
  const ready = kind === "platform" || kind === "tenant" || !!target;
  const allowed = (action: string) =>
    platform ||
    !!caps[action]?.all ||
    (
      (kind === "team"
        ? caps[action]?.teams
        : kind === "space"
          ? caps[action]?.spaces
          : caps[action]?.apps) || []
    ).includes(target) ||
    (kind === "app" &&
      (caps[action]?.spaces || []).includes(resource.space_id));
  const managerAssignable = (role: Item) =>
    role.domain !== "control" ||
    kind === "tenant" ||
    kind === "team" ||
    platform ||
    !!caps["authorization.manage"]?.all ||
    (kind === "app" &&
      (caps["authorization.manage"]?.spaces || []).includes(resource.space_id));
  const manage = ready && allowed("authorization.manage");
  const teamManage = kind === "team" && ready && allowed("team.manage");
  const directory = platform || !!caps["membership.read"]?.all;
  const set = (key: string, value: any) =>
    setValues((v) => ({ ...v, [key]: value }));
  useEffect(() => {
    setKind(defaultKind());
    setTarget(scopeId || "");
    setForm("");
    setPage(1);
    setQuery("");
    setView("assignments");
    setResource({});
  }, [tenant, scopeKind, scopeId]);
  useEffect(() => {
    setForm("");
    setPage(1);
    setQuery("");
    setRecords([]);
    setMembers([]);
    setMemberPage(1);
    setMemberQuery("");
    setResource({});
  }, [kind, target, view]);
  useEffect(() => {
    if (!ready) {
      setRecords([]);
      return;
    }
    let live = true;
    setLoading(true);
    setError("");
    const timer = setTimeout(async () => {
      try {
        let meta: Item = {};
        if (kind === "team" || kind === "app") meta = await api(path);
        let list: Item;
        const defs =
          kind === "platform"
            ? (await api("/v1/roles/catalog?scope_kind=platform")).items
            : (await api(path + "/roles?limit=200")).items;
        if (view === "roles")
          list = await api(
            path + `/roles?page=${page}&q=${encodeURIComponent(query)}`,
          );
        else
          list = await api(
            path +
              `/role-assignments?page=${page}&q=${encodeURIComponent(query)}`,
          );
        let teamMembers: Item[] = [];
        if (
          kind === "team" &&
          (platform ||
            caps["team.manage"]?.all ||
            (caps["team.manage"]?.teams || []).includes(target))
        ) {
          const result = await api(
            path +
              `/members?page=${memberPage}&q=${encodeURIComponent(memberQuery)}`,
          );
          teamMembers = result.items;
          if (live) setMemberMore(!!result.pagination?.has_more);
        }
        if (live) {
          setResource(meta);
          setRoles(defs);
          setRecords(list.items);
          setMembers(teamMembers);
          setMore(!!list.pagination?.has_more);
        }
      } catch (e) {
        if (live) setError((e as Error).message);
      } finally {
        if (live) setLoading(false);
      }
    }, 150);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [
    tenant,
    kind,
    target,
    path,
    ready,
    view,
    page,
    query,
    revision,
    platform,
    caps,
    memberPage,
    memberQuery,
  ]);
  function open(type: string, item?: Item) {
    setEditing(item || null);
    setValues(
      item
        ? { display_name: item.display_name, state: item.state }
        : { principal_kind: "user", state: "active" },
    );
    setForm(type);
    setError("");
  }
  function changed() {
    setForm("");
    setRevision((v) => v + 1);
    refresh();
  }
  async function act(fn: () => Promise<any>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await fn();
      changed();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function save(event: React.FormEvent) {
    event.preventDefault();
    await act(async () => {
      if (form === "role")
        return api(
          path + "/roles" + (editing ? "/" + editing.id : ""),
          editing ? "PATCH" : "POST",
          editing
            ? { display_name: values.display_name, state: values.state }
            : {
                id: crypto.randomUUID(),
                code: values.code,
                display_name: values.display_name,
                state: values.state,
              },
          editing?.revision,
        );
      if (form === "assignment")
        return api(path + "/role-assignments", "POST", {
          id: crypto.randomUUID(),
          role_id: values.role_id,
          ...(values.principal_kind === "team"
            ? { group_id: values.group_id }
            : { user_id: values.user_id }),
        });
      if (form === "team") {
        if (editing)
          return api(
            path,
            "PATCH",
            { display_name: values.display_name, state: values.state },
            editing.revision,
          );
        const id = crypto.randomUUID();
        await api(base + "/teams", "POST", {
          id,
          slug: values.slug,
          display_name: values.display_name,
        });
        setTarget(id);
        return;
      }
      if (form === "member")
        return api(
          path + "/members",
          "POST",
          { user_id: values.user_id },
          resource.revision,
        );
    });
  }
  function picker(
    label: string,
    key: string,
    lookup?: string,
    valueKey = "id",
  ) {
    return (
      <label>
        {label}
        {lookup ? (
          <RemoteSelect
            key={lookup}
            path={lookup}
            label={label}
            value={values[key] || ""}
            onChange={(v) => set(key, v)}
            initial={[]}
            required
            valueKey={valueKey}
            excludeInactive
            excludePlatform={
              form === "assignment" &&
              kind !== "platform" &&
              roles.find((r) => r.id === values.role_id)?.domain === "control"
            }
          />
        ) : (
          <input
            aria-label={label}
            required
            value={values[key] || ""}
            onChange={(e) => set(key, e.target.value)}
            placeholder="Existing member/team UUID"
          />
        )}
      </label>
    );
  }
  return (
    <section className="panel access-foundation">
      <p className="access-intro">
        Fixed Tenant and team roles provide administration and team
        relationships. Space/App business roles are reusable foundations;
        business-object ACLs will be added with the entity API.
      </p>
      {tenant && !scopeKind && (
        <div className="tabs">
          {scopes.map(([id, label]) => (
            <button
              key={id}
              className={kind === id ? "active" : ""}
              onClick={() => {
                setTarget("");
                setKind(id);
                setView("assignments");
              }}
            >
              {label}
            </button>
          ))}
        </div>
      )}
      {tenant && kind !== "tenant" && !scopeId && (
        <div className="access-scope">
          <RemoteSelect
            key={kind + tenant}
            path={
              base +
              "/" +
              (kind === "team" ? "teams" : kind === "space" ? "spaces" : "apps")
            }
            label={
              kind === "team"
                ? "Team scope"
                : kind === "space"
                  ? "Space scope"
                  : "App scope"
            }
            value={target}
            onChange={setTarget}
            initial={[]}
          />
          {kind === "team" && (platform || caps["team.manage"]?.all) && (
            <button className="secondary" onClick={() => open("team")}>
              Create team
            </button>
          )}
        </div>
      )}
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      {ready || form === "team" ? (
        <>
          <div className="toolbar access-toolbar">
            <div className="tabs">
              <button
                className={view === "assignments" ? "active" : ""}
                onClick={() => setView("assignments")}
              >
                Role assignments
              </button>
              {kind !== "platform" && (
                <button
                  className={view === "roles" ? "active" : ""}
                  onClick={() => setView("roles")}
                >
                  Roles
                </button>
              )}
            </div>
            <div className="row-actions">
              {teamManage && (
                <>
                  <button
                    className="secondary"
                    disabled={loading || resource.id !== target}
                    onClick={() => open("team", resource)}
                  >
                    Edit team
                  </button>
                  <button className="secondary" onClick={() => open("member")}>
                    Add team member
                  </button>
                </>
              )}
              {manage && (
                <button
                  className="primary"
                  onClick={() =>
                    open(
                      view === "roles" && (kind === "space" || kind === "app")
                        ? "role"
                        : "assignment",
                    )
                  }
                >
                  {view === "roles" && (kind === "space" || kind === "app")
                    ? "Create role"
                    : "Assign role"}
                </button>
              )}
            </div>
          </div>
          {kind === "tenant" && (
            <p className="muted">
              Tenant roles are fixed. Assign Tenant administration to an
              individual member.
            </p>
          )}
          {kind === "team" && (
            <p className="muted">
              A role here belongs to one person within this team. Membership
              alone does not grant Reviewer.
            </p>
          )}
          {kind === "app" && (
            <p className="muted">
              Apps reuse Space business roles by default. App roles are optional
              additions; they do not widen Space authority or grant management
              access.
            </p>
          )}
          {form && (
            <form
              className="access-editor"
              aria-label={
                form === "role"
                  ? editing
                    ? "Edit role"
                    : "Create role"
                  : form === "assignment"
                    ? "Assign role"
                    : form === "member"
                      ? "Add team member"
                      : editing
                        ? "Edit team"
                        : "Create team"
              }
              onSubmit={save}
            >
              <h3>
                {form === "role"
                  ? editing
                    ? "Edit business role"
                    : "Create business role"
                  : form === "assignment"
                    ? "Assign a role"
                    : form === "member"
                      ? "Add a team member"
                      : editing
                        ? "Edit team"
                        : "Create team"}
              </h3>
              <div className="form-grid">
                {(form === "role" || form === "team") && (
                  <>
                    <label>
                      Name
                      <input
                        aria-label="Name"
                        required
                        maxLength={160}
                        value={values.display_name || ""}
                        onChange={(e) => set("display_name", e.target.value)}
                      />
                    </label>
                    {!editing && (
                      <label>
                        {form === "role" ? "Code" : "Slug"}
                        <input
                          aria-label={form === "role" ? "Code" : "Slug"}
                          pattern="[a-z][a-z0-9-]{0,62}"
                          required
                          value={values.code || values.slug || ""}
                          onChange={(e) =>
                            set(
                              form === "role" ? "code" : "slug",
                              e.target.value,
                            )
                          }
                        />
                      </label>
                    )}
                    {(editing || form === "role") && (
                      <label>
                        State
                        <select
                          aria-label="State"
                          value={values.state}
                          onChange={(e) => set("state", e.target.value)}
                        >
                          <option value="active">Active</option>
                          <option value="disabled">Disabled</option>
                        </select>
                      </label>
                    )}
                    {form === "role" && (
                      <p className="muted">
                        Business roles cannot contain administrative
                        permissions. Entity permissions will use the later ACL
                        contract.
                      </p>
                    )}
                  </>
                )}
                {form === "assignment" && (
                  <>
                    <label>
                      Role
                      <RemoteSelect
                        path={
                          kind === "platform"
                            ? "/v1/roles/catalog?scope_kind=platform"
                            : path + "/roles"
                        }
                        label="Role"
                        value={values.role_id || ""}
                        onChange={(v) =>
                          setValues((old) => ({
                            ...old,
                            role_id: v,
                            principal_kind:
                              roles.find((r) => r.id === v)?.domain ===
                              "control"
                                ? "user"
                                : old.principal_kind,
                          }))
                        }
                        initial={roles
                          .filter(managerAssignable)
                          .map((r) => ({ value: r.id, label: r.display_name }))}
                        required
                        excludeInactive
                        excludeControl={
                          !managerAssignable({ domain: "control" })
                        }
                      />
                    </label>
                    {(kind === "space" || kind === "app") &&
                      roles.find((r) => r.id === values.role_id)?.domain !==
                        "control" && (
                        <label>
                          Recipient type
                          <select
                            aria-label="Recipient type"
                            value={values.principal_kind}
                            onChange={(e) =>
                              set("principal_kind", e.target.value)
                            }
                          >
                            <option value="user">User</option>
                            <option value="team">Team</option>
                          </select>
                        </label>
                      )}
                    {values.principal_kind === "team" &&
                    (kind === "space" || kind === "app")
                      ? picker(
                          "Recipient team",
                          "group_id",
                          platform || caps["team.read"]?.all
                            ? base + "/teams"
                            : undefined,
                        )
                      : picker(
                          "User",
                          "user_id",
                          kind === "platform"
                            ? "/admin/v1/users"
                            : kind === "team"
                              ? path + "/members"
                              : directory
                                ? base + "/memberships"
                                : undefined,
                          kind === "platform" ? "id" : "user_id",
                        )}
                  </>
                )}
                {form === "member" &&
                  picker(
                    "Tenant member",
                    "user_id",
                    directory ? base + "/memberships" : undefined,
                    "user_id",
                  )}
              </div>
              <div className="row-actions">
                <button
                  type="button"
                  className="secondary"
                  onClick={() => setForm("")}
                  disabled={busy}
                >
                  Cancel
                </button>
                <button className="primary" disabled={busy}>
                  {busy ? "Saving…" : "Save changes"}
                </button>
              </div>
            </form>
          )}
          <div className="toolbar">
            <label className="search">
              <Search size={16} aria-hidden="true" />
              <input
                placeholder="Search roles or assignments…"
                aria-label="Search records"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setPage(1);
                }}
              />
            </label>
            <span>{loading ? "Loading…" : `${records.length} records`}</span>
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  {(view === "roles"
                    ? ["Role", "Category", "Definition", "State", "Actions"]
                    : ["Recipient", "Role", "Scope", "Actions"]
                  ).map((x) => (
                    <th key={x}>{x}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {records.map((row) => (
                  <tr key={row.id}>
                    {view === "roles" ? (
                      <>
                        <td>
                          {row.display_name}
                          <small>{row.code}</small>
                        </td>
                        <td>
                          {row.domain === "control"
                            ? "Administration / team"
                            : "Business"}
                        </td>
                        <td>
                          {row.protected ? (
                            <details>
                              <summary>Platform defined</summary>
                              <ul>
                                {row.permissions.map((p: string) => (
                                  <li key={p}>
                                    <code>{p}</code>
                                  </li>
                                ))}
                              </ul>
                            </details>
                          ) : (
                            "Custom business role"
                          )}
                        </td>
                        <td>{row.state}</td>
                        <td>
                          {row.protected ? (
                            <span className="muted">Fixed</span>
                          ) : (
                            manage && (
                              <button
                                className="text-button"
                                onClick={() => open("role", row)}
                              >
                                Edit role
                              </button>
                            )
                          )}
                        </td>
                      </>
                    ) : (
                      <>
                        <td>
                          {row.principal_name}
                          <small>
                            {row.group_id ? "Team recipient" : "User"}
                          </small>
                        </td>
                        <td>{row.access_name}</td>
                        <td>{row.scope_name}</td>
                        <td>
                          {manage &&
                            managerAssignable({
                              id: row.role_id,
                              domain: row.domain,
                            }) && (
                              <button
                                className="text-button danger"
                                disabled={busy}
                                onClick={() => {
                                  if (
                                    window.confirm(
                                      `Revoke ${row.access_name} from ${row.principal_name}?`,
                                    )
                                  )
                                    act(() =>
                                      api(
                                        path + "/role-assignments/" + row.id,
                                        "DELETE",
                                        undefined,
                                        row.revision,
                                      ),
                                    );
                                }}
                              >
                                Revoke
                              </button>
                            )}
                        </td>
                      </>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {!loading && !records.length && (
            <p className="empty">
              No {view === "roles" ? "roles" : "role assignments"} in this
              scope.
            </p>
          )}
          <div className="pagination">
            <button
              className="secondary"
              aria-label="Previous records"
              disabled={page === 1 || loading}
              onClick={() => setPage((p) => p - 1)}
            >
              Previous
            </button>
            <span>Page {page}</span>
            <button
              className="secondary"
              aria-label="Next records"
              disabled={!more || loading}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </button>
          </div>
          {teamManage && (
            <section className="access-members">
              <h3>Team membership</h3>
              <p className="muted">
                Disabling preserves role assignments. Removing a member revokes
                their team-scoped roles.
              </p>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Member</th>
                      <th>State</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {members.map((m) => (
                      <tr key={m.user_id}>
                        <td>{m.display_name}</td>
                        <td>
                          {m.state}
                          {m.user_state !== "active"
                            ? " · user disabled"
                            : m.tenant_state !== "active"
                              ? " · Tenant membership disabled"
                              : ""}
                        </td>
                        <td>
                          <div className="row-actions">
                            <button
                              className="text-button"
                              disabled={busy}
                              onClick={() =>
                                act(() =>
                                  api(
                                    path + "/members/" + m.user_id,
                                    "PATCH",
                                    {
                                      state:
                                        m.state === "active"
                                          ? "disabled"
                                          : "active",
                                    },
                                    resource.revision,
                                  ),
                                )
                              }
                            >
                              {m.state === "active" ? "Disable" : "Enable"}
                            </button>
                            <button
                              className="text-button danger"
                              disabled={busy}
                              onClick={() => {
                                if (
                                  window.confirm(
                                    `Remove ${m.display_name} and revoke their team roles?`,
                                  )
                                )
                                  act(() =>
                                    api(
                                      path + "/members/" + m.user_id,
                                      "DELETE",
                                      undefined,
                                      resource.revision,
                                    ),
                                  );
                              }}
                            >
                              Remove
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="toolbar">
                <label>
                  Find team member
                  <input
                    aria-label="Find team member"
                    value={memberQuery}
                    onChange={(e) => {
                      setMemberPage(1);
                      setMemberQuery(e.target.value);
                    }}
                  />
                </label>
                <div className="row-actions">
                  <button
                    className="secondary"
                    aria-label="Previous team members"
                    disabled={memberPage === 1 || loading}
                    onClick={() => setMemberPage((p) => p - 1)}
                  >
                    Previous
                  </button>
                  <span>Page {memberPage}</span>
                  <button
                    className="secondary"
                    aria-label="Next team members"
                    disabled={!memberMore || loading}
                    onClick={() => setMemberPage((p) => p + 1)}
                  >
                    Next
                  </button>
                </div>
              </div>
            </section>
          )}
        </>
      ) : (
        <p className="empty">
          Choose a {kind} to manage its roles and assignments.
        </p>
      )}
    </section>
  );
}
