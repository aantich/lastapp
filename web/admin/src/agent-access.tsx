import { useEffect, useState, useRef } from "react";
import { Bot, X } from "lucide-react";
import { api } from "./api";
import { RemoteSelect } from "./remote-select";
import { Select } from "./select";
import { NumberInput } from "./number-input";
import { useDialogFocus } from "./dialog";
import { errorMessage } from "./use-action";

import type { Item, Page, Redirect } from "./api-types";
type GrantValues = {
  tenant_id: string;
  scope_kind: string;
  scope_id: string;
  permissions: string[];
  limits: { target_ids: string[]; max_databases: number };
  days: number;
};
function initialGrant(config: Item, me: Item, grant?: Item): GrantValues {
  return {
    tenant_id:
      grant?.tenant_id || (me.platform_admin ? "" : me.tenants?.[0]?.id || ""),
    scope_kind:
      grant?.scope_kind || (me.platform_admin ? "platform" : "tenant"),
    scope_id: grant?.scope_id || "",
    permissions: grant?.permissions_json || config.default_permissions,
    limits: grant?.limits_json || config.default_limits,
    days: config.default_days,
  };
}
function AgentGrantForm({
  config,
  me,
  initial,
  requested,
  locked,
  submit,
  cancel,
  label,
}: {
  config: Item;
  me: Item;
  initial: GrantValues;
  requested?: string[];
  locked?: boolean;
  submit: (v: GrantValues) => Promise<void>;
  cancel: () => void;
  label: string;
}) {
  const [values, setValues] = useState(initial),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [targetID, setTargetID] = useState("");
  const eligiblePermissions = (kind: string) =>
    config.permissions.filter(
      (p: Item) =>
        (!requested || requested.includes(p.code)) &&
        p.scope_kinds.includes(kind),
    );
  const permissionList = eligiblePermissions(values.scope_kind);
  const provisioning = values.permissions.includes("databases.provision");
  const scopePath =
    values.scope_kind === "space"
      ? "spaces"
      : values.scope_kind === "app"
        ? "apps"
        : "teams";
  function permissions(codes: string[]) {
    setValues({
      ...values,
      permissions: codes.filter((code) =>
        permissionList.some((p: Item) => p.code === code),
      ),
    });
  }
  return (
    <form
      className="fields agent-grant-form"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setError("");
        try {
          await submit(values);
        } catch (e) {
          setError(errorMessage(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      {!locked && (
        <div className="agent-scope-grid">
          {!me.platform_admin && (
            <div className="form-field">
              <span className="field-label">Tenant</span>
              <RemoteSelect
                path="/v1/my-tenants"
                label="Tenant"
                value={values.tenant_id}
                required
                initial={(me.tenants || []).map((t: Item) => ({
                  value: t.id,
                  label: t.display_name,
                }))}
                onChange={(id) =>
                  setValues({ ...values, tenant_id: id, scope_id: "" })
                }
              />
            </div>
          )}
          <label className="form-field">
            <span className="field-label">Access scope</span>
            <Select
              value={values.scope_kind}
              onChange={(scope_kind) =>
                setValues({
                  ...values,
                  scope_kind,
                  scope_id: "",
                  permissions: values.permissions.filter((code) =>
                    eligiblePermissions(scope_kind).some(
                      (p: Item) => p.code === code,
                    ),
                  ),
                })
              }
              options={(me.platform_admin
                ? ["platform"]
                : ["tenant", "space", "app", "team"]
              ).map((kind) => ({
                value: kind,
                label:
                  kind === "tenant"
                    ? "Entire Tenant, including future resources"
                    : kind[0].toUpperCase() + kind.slice(1),
              }))}
            />
          </label>
          {!["tenant", "platform"].includes(values.scope_kind) &&
            values.tenant_id && (
              <div className="form-field agent-span">
                <span className="field-label">Choose {values.scope_kind}</span>
                <RemoteSelect
                  key={scopePath + values.tenant_id}
                  path={`/v1/tenants/${values.tenant_id}/${scopePath}`}
                  label={`Choose ${values.scope_kind}`}
                  required
                  value={values.scope_id}
                  initial={[]}
                  onChange={(id) => setValues({ ...values, scope_id: id })}
                />
              </div>
            )}
        </div>
      )}
      {locked && (
        <p className="muted">
          Bound to {values.scope_kind}
          {values.scope_id ? ` · ${values.scope_id}` : ""}. Reconnect to choose
          a different scope.
        </p>
      )}
      <fieldset className="agent-permissions">
        <legend>What may this agent do?</legend>
        <div className="agent-presets">
          {config.presets
            .filter((preset: Item) =>
              preset.permissions.every((code: string) =>
                permissionList.some((p: Item) => p.code === code),
              ),
            )
            .map((preset: Item) => (
              <button
                key={preset.label}
                type="button"
                className="secondary"
                onClick={() => permissions(preset.permissions)}
              >
                {preset.label}
              </button>
            ))}
        </div>
        {permissionList.map((permission: Item) => (
          <label className="agent-permission" key={permission.code}>
            <input
              type="checkbox"
              checked={values.permissions.includes(permission.code)}
              onChange={(e) =>
                permissions(
                  e.target.checked
                    ? [...values.permissions, permission.code]
                    : values.permissions.filter((p) => p !== permission.code),
                )
              }
            />
            <span>
              <strong>{permission.label}</strong>
              <small>{permission.description}</small>
            </span>
          </label>
        ))}
      </fieldset>
      {provisioning && (
        <fieldset className="agent-provisioning">
          <legend>Provisioning limits</legend>
          <p className="muted">
            Only these targets may receive new databases. The count includes
            allocations made during Space creation.
          </p>
          <RemoteSelect
            path={`/v1/tenants/${values.tenant_id}/database-targets?managed=true`}
            label="Approved database target"
            value={targetID}
            initial={[]}
            onChange={setTargetID}
          />
          <button
            type="button"
            className="secondary"
            disabled={
              !targetID ||
              values.limits.target_ids.includes(targetID) ||
              values.limits.target_ids.length >= config.max_targets
            }
            onClick={() => {
              setValues({
                ...values,
                limits: {
                  ...values.limits,
                  target_ids: [...values.limits.target_ids, targetID],
                },
              });
              setTargetID("");
            }}
          >
            Approve target
          </button>
          {values.limits.target_ids.map((id) => (
            <div className="agent-target" key={id}>
              <code>{id}</code>
              <button
                type="button"
                aria-label={`Remove target ${id}`}
                className="icon"
                onClick={() =>
                  setValues({
                    ...values,
                    limits: {
                      ...values.limits,
                      target_ids: values.limits.target_ids.filter(
                        (v) => v !== id,
                      ),
                    },
                  })
                }
              >
                <X size={16} />
              </button>
            </div>
          ))}
          <label className="form-field">
            <span className="field-label">Maximum new databases</span>
            <NumberInput
              min={1}
              max={config.max_databases}
              required
              steppers
              value={values.limits.max_databases}
              onChange={(n) =>
                setValues({
                  ...values,
                  limits: {
                    ...values.limits,
                    max_databases: Number(n),
                  },
                })
              }
            />
          </label>
        </fieldset>
      )}
      <label className="form-field agent-expiry">
        <span className="field-label">Access expires in days</span>
        <NumberInput
          min={1}
          max={config.max_days}
          required
          steppers
          value={values.days}
          onChange={(n) => setValues({ ...values, days: Number(n) })}
        />
        <small className="muted">
          You can revoke access at any time. Closing the browser or signing out
          does not revoke this connection.
        </small>
      </label>
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      <div className="agent-form-actions">
        <button
          type="button"
          className="secondary"
          onClick={cancel}
          disabled={busy}
        >
          Cancel
        </button>
        <button
          className="primary"
          disabled={busy || values.permissions.length === 0}
        >
          {busy ? "Saving…" : label}
        </button>
      </div>
    </form>
  );
}

export function AgentConsent({ me }: { me: Item }) {
  const [info, setInfo] = useState<Item | null>(null),
    [config, setConfig] = useState<Item | null>(null),
    [error, setError] = useState("");
  const request = new URLSearchParams(location.search).get("request") || "";
  useEffect(() => {
    let live = true;
    Promise.all([
      api<Item>(`/v1/agent-access/requests/${encodeURIComponent(request)}`),
      api<Item>("/v1/agent-access/config"),
    ])
      .then(([i, c]) => {
        if (live) {
          setInfo(i);
          setConfig(c);
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [request]);
  async function deny() {
    try {
      const result = await api<Redirect>(`/v1/agent-access/requests/${request}`, "POST", {
        deny: true,
      });
      location.assign(result.redirect_url);
    } catch (e) {
      setError(errorMessage(e));
    }
  }
  return (
    <main className="agent-consent-page">
      <section className="panel agent-consent-card">
        <div className="agent-heading">
          <span className="brand-mark">
            <Bot size={24} />
          </span>
          <div>
            <h1>Connect an agent</h1>
            <p>Choose exactly what this client may do as {me.display_name}.</p>
          </div>
        </div>
        {error && (
          <div className="error" role="alert">
            {error}
          </div>
        )}
        {info && config ? (
          <>
            <div className="agent-client">
              <strong>{info.client_name}</strong>
              <code>{info.client_id}</code>
              <small>
                Check this client identity before approving. Its display name is
                provided by the client.
              </small>
              <small>Returns to {new URL(info.redirect_uri).host}</small>
            </div>
            <p className="muted">
              Configuration management only. This connection cannot read or
              change business records or retrieve credentials.
            </p>
            <AgentGrantForm
              config={config}
              me={me}
              initial={{
                ...initialGrant(config, me),
                permissions: config.default_permissions.filter((code: string) =>
                  info.scopes.split(" ").includes(code),
                ),
              }}
              requested={info.scopes.split(" ")}
              submit={async (values) => {
                const result = await api<Redirect>(
                  `/v1/agent-access/requests/${request}`,
                  "POST",
                  values,
                );
                location.assign(result.redirect_url);
              }}
              cancel={deny}
              label="Approve agent access"
            />
          </>
        ) : (
          !error && <p className="muted">Loading connection request…</p>
        )}
      </section>
    </main>
  );
}

function AgentGrantDialog({
  grant,
  config,
  me,
  close,
  done,
}: {
  grant: Item;
  config: Item;
  me: Item;
  close: () => void;
  done: () => void;
}) {
  const ref = useDialogFocus(close);
  return (
    <div className="overlay">
      <section
        className="modal agent-edit-modal"
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby="agent-dialog-title"
      >
        <header>
          <div>
            <h2 id="agent-dialog-title">Agent access</h2>
            <p>{grant.client_name}</p>
          </div>
          <button
            className="icon"
            aria-label="Close agent access"
            onClick={close}
          >
            <X />
          </button>
        </header>
        <div className="modal-body">
          <AgentGrantForm
            config={config}
            me={me}
            initial={initialGrant(config, me, grant)}
            locked
            submit={async (values) => {
              await api(
                `/v1/agent-access/${grant.id}`,
                "PATCH",
                values,
                grant.revision,
              );
              done();
            }}
            cancel={close}
            label="Save access"
          />
        </div>
      </section>
    </div>
  );
}
export function AgentAccess({ me, tenant }: { me: Item; tenant?: string }) {
  const [config, setConfig] = useState<Item | null>(null),
    [items, setItems] = useState<Item[]>([]),
    [page, setPage] = useState(1),
    [more, setMore] = useState(false),
    [error, setError] = useState(""),
    [editing, setEditing] = useState<Item | null>(null),
    [loading, setLoading] = useState(true),
    [loadFailed, setLoadFailed] = useState(false),
    [busy, setBusy] = useState(""),
    [copied, setCopied] = useState(false);
  const generation = useRef(0);
  async function load() {
    const request = ++generation.current;
    setLoading(true);
    try {
      const [c, list] = await Promise.all([
        api<Item>("/v1/agent-access/config"),
        api<Page>(
          `/v1/agent-access?page=${page}&limit=20${tenant ? "&tenant_id=" + tenant : ""}`,
        ),
      ]);
      if (request !== generation.current) return;
      setConfig(c);
      setItems(list.items);
      setMore(!!list.pagination?.has_more);
      setLoadFailed(false);
      setError("");
    } catch (e) {
      if (request === generation.current) {
        setLoadFailed(true);
        setError(errorMessage(e));
      }
    } finally {
      if (request === generation.current) setLoading(false);
    }
  }
  useEffect(() => {
    load();
    return () => {
      generation.current++;
    };
  }, [page, tenant]);
  return (
    <section className="panel workspace-section agent-access">
      <div className="settings-heading">
        <div className="settings-summary">
          <h2>Agent access</h2>
          <p>
            {tenant
              ? "Review connections approved by members of this Tenant and revoke access when needed."
              : "Review the clients acting on your behalf and revoke them when finished."}
          </p>
        </div>
        <button className="secondary" disabled={loading} onClick={load}>
          Refresh connections
        </button>
      </div>
      {config && (
        <div className="agent-endpoint">
          <div className="agent-endpoint-heading">
            <strong>Connect from Codex or Claude</strong>
            <button
              type="button"
              className="secondary"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(config.mcp_url);
                  setCopied(true);
                } catch {
                  setError("Select and copy the MCP URL below.");
                }
              }}
            >
              {copied ? "URL copied" : "Copy MCP URL"}
            </button>
          </div>
          <code>{config.mcp_url}</code>
          <small>
            Add this URL in your agent client. It opens Lastapp sign-in and asks
            you to approve access. No API keys or OAuth client setup required.
          </small>
        </div>
      )}
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      {loading ? (
        <p className="muted">Loading agent connections…</p>
      ) : loadFailed ? null : items.length === 0 ? (
        <div className="agent-empty">
          <Bot size={26} />
          <p>No agent connections yet.</p>
          <span className="muted">
            Add the MCP endpoint to your agent client to begin.
          </span>
        </div>
      ) : (
        <div className="agent-connection-list">
          {items.map((grant) => (
            <article className="agent-connection" key={grant.id}>
              <div className="agent-connection-info">
                <strong>{grant.client_name}</strong>
                <small>
                  {grant.scope_name} · {grant.scope_kind}
                </small>
                <span className="muted">
                  {grant.permissions_json
                    .map(
                      (code: string) =>
                        config?.permissions.find(
                          (permission: Item) => permission.code === code,
                        )?.label || code,
                    )
                    .join(" · ")}
                </span>
                {tenant && <small>Approved by {grant.owner_name}</small>}
                <small>
                  Expires {new Date(grant.expires_at).toLocaleString()} ·{" "}
                  {grant.state}
                </small>
                {grant.last_used_at && (
                  <small>
                    Last mutation{" "}
                    {new Date(grant.last_used_at).toLocaleString()}
                  </small>
                )}
              </div>
              <div className="row-actions">
                {grant.state === "active" && (
                  <>
                    {grant.user_id === me.id && (
                      <button
                        className="secondary"
                        onClick={() => setEditing(grant)}
                      >
                        Edit access
                      </button>
                    )}
                    <button
                      className="danger"
                      disabled={busy === grant.id}
                      onClick={async () => {
                        setBusy(grant.id);
                        try {
                          await api(
                            `/v1/agent-access/${grant.id}`,
                            "PATCH",
                            { state: "revoked" },
                            grant.revision,
                          );
                          await load();
                        } catch (e) {
                          setError(errorMessage(e));
                        } finally {
                          setBusy("");
                        }
                      }}
                    >
                      Revoke access
                    </button>
                  </>
                )}
              </div>
            </article>
          ))}
        </div>
      )}
      {(page > 1 || more) && (
        <div className="agent-pagination">
          <button
            className="secondary"
            disabled={page === 1}
            onClick={() => setPage(page - 1)}
          >
            Previous
          </button>
          <span>Page {page}</span>
          <button
            className="secondary"
            disabled={!more}
            onClick={() => setPage(page + 1)}
          >
            Next
          </button>
        </div>
      )}
      {editing && config && (
        <AgentGrantDialog
          key={editing.id}
          grant={editing}
          config={config}
          me={me}
          close={() => setEditing(null)}
          done={() => {
            setEditing(null);
            load();
          }}
        />
      )}
    </section>
  );
}
