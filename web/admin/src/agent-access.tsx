import { useState, type ReactNode } from "react";
import { Bot, Copy, RefreshCw, X } from "lucide-react";
import { api } from "./api";
import {
  addDays,
  Alert,
  Avatar,
  Badge,
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  Checkbox,
  CodeBlock,
  Columns,
  confirmAction,
  DatePicker,
  daysBetween,
  Dialog,
  DialogBody,
  DialogFooter,
  EmptyState,
  FactList,
  FieldGroup,
  FormField,
  IconButton,
  Inline,
  isoDay,
  NumberInput,
  PageHeader,
  parseDay,
  RemoteSelect,
  SegmentedControl,
  Select,
  Skeleton,
  Stack,
  StandalonePage,
  StatusBadge,
} from "./ui";
import { useAction } from "./use-action";
import { refreshQueries, useApiQuery } from "./query";

import type {
  AgentConfig,
  AgentGrant,
  AgentLimits,
  AgentRequest,
  Me,
  Page,
  Redirect,
} from "./api-types";
const agentConfigPath = "/v1/agent-access/config";
// Quick expiry choices, offered only where the backend's max_days allows.
const quickDays = [7, 30, 90];
type GrantValues = {
  tenant_id: string;
  scope_kind: string;
  scope_id: string;
  permissions: string[];
  limits: AgentLimits;
  days: number;
};
function initialGrant(
  config: AgentConfig,
  me: Me,
  grant?: AgentGrant,
): GrantValues {
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
const sameSet = (a: string[], b: string[]) =>
  a.length === b.length && a.every((code) => b.includes(code));
const when = (iso: string) => new Date(iso).toLocaleString();

// The scope → preset → permissions → limits → expiry steps shared by the
// consent page (in a card) and the edit dialog. The backend contract stays
// a number of days; the date picker is a view of it, bounded by max_days.
function AgentGrantForm({
  config,
  me,
  initial,
  requested,
  locked,
  intro,
  inDialog,
  submit,
  cancel,
  label,
}: {
  config: AgentConfig;
  me: Me;
  initial: GrantValues;
  requested?: string[];
  locked?: boolean;
  intro?: ReactNode;
  inDialog?: boolean;
  submit: (v: GrantValues) => Promise<void>;
  cancel: () => void;
  label: string;
}) {
  const [values, setValues] = useState(initial),
    [error, setError] = useState(""),
    [targetID, setTargetID] = useState("");
  const save = useAction(label, { onSuccess: () => {}, onError: setError });
  const eligiblePermissions = (kind: string) =>
    config.permissions.filter(
      (p) =>
        (!requested || requested.includes(p.code)) &&
        p.scope_kinds.includes(kind),
    );
  const permissionList = eligiblePermissions(values.scope_kind);
  const presets = config.presets.filter((preset) =>
    preset.permissions.every((code) =>
      permissionList.some((p) => p.code === code),
    ),
  );
  const provisioning = values.permissions.includes("databases.provision");
  const scopePath =
    values.scope_kind === "space"
      ? "spaces"
      : values.scope_kind === "app"
        ? "apps"
        : "teams";
  const today = new Date(),
    expiry = isoDay(addDays(today, values.days)),
    quick = quickDays.filter((d) => d <= config.max_days);
  function permissions(codes: string[]) {
    setValues({
      ...values,
      permissions: codes.filter((code) =>
        permissionList.some((p) => p.code === code),
      ),
    });
  }
  const Body = inDialog ? DialogBody : CardBody,
    Footer = inDialog ? DialogFooter : CardFooter;
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void save.run(() => submit(values));
      }}
    >
      <Body>
        <Stack gap="md" className="agent-grant-form">
          {intro}
          {locked ? (
            <Alert tone="info" live={false}>
              Bound to {values.scope_kind}
              {values.scope_id ? ` · ${values.scope_id}` : ""}. Reconnect to
              choose a different scope.
            </Alert>
          ) : (
            <Columns min={220} gap="md">
              {!me.platform_admin && (
                <FormField label="Tenant" required>
                  <RemoteSelect
                    path="/v1/my-tenants"
                    label="Tenant"
                    value={values.tenant_id}
                    required
                    initial={(me.tenants || []).map((t) => ({
                      value: t.id,
                      label: t.display_name,
                    }))}
                    onChange={(id) =>
                      setValues({ ...values, tenant_id: id, scope_id: "" })
                    }
                  />
                </FormField>
              )}
              <FormField label="Access scope">
                <Select
                  value={values.scope_kind}
                  onChange={(scope_kind) =>
                    setValues({
                      ...values,
                      scope_kind,
                      scope_id: "",
                      permissions: values.permissions.filter((code) =>
                        eligiblePermissions(scope_kind).some(
                          (p) => p.code === code,
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
              </FormField>
              {!["tenant", "platform"].includes(values.scope_kind) &&
                values.tenant_id && (
                  <FormField label={`Choose ${values.scope_kind}`} required>
                    <RemoteSelect
                      key={scopePath + values.tenant_id}
                      path={`/v1/tenants/${values.tenant_id}/${scopePath}`}
                      label={`Choose ${values.scope_kind}`}
                      required
                      value={values.scope_id}
                      initial={[]}
                      onChange={(id) => setValues({ ...values, scope_id: id })}
                    />
                  </FormField>
                )}
            </Columns>
          )}
          <FieldGroup
            framed
            legend="What may this agent do?"
            description="Presets only tick boxes; the agent gets exactly the checked permissions."
          >
            {presets.length > 0 && (
              <SegmentedControl
                label="Permission presets"
                options={presets.map((p) => ({
                  value: p.label,
                  label: p.label,
                }))}
                value={
                  presets.find((p) =>
                    sameSet(p.permissions, values.permissions),
                  )?.label || ""
                }
                onChange={(preset) =>
                  permissions(
                    presets.find((p) => p.label === preset)?.permissions || [],
                  )
                }
              />
            )}
            {permissionList.map((permission) => (
              <Checkbox
                key={permission.code}
                label={permission.label}
                description={permission.description}
                checked={values.permissions.includes(permission.code)}
                onChange={(checked) =>
                  permissions(
                    checked
                      ? [...values.permissions, permission.code]
                      : values.permissions.filter((p) => p !== permission.code),
                  )
                }
              />
            ))}
          </FieldGroup>
          {provisioning && (
            <FieldGroup
              framed
              legend="Provisioning limits"
              description="Only these targets may receive new databases. The count includes allocations made during Space creation."
            >
              <FormField label="Approved database target">
                <RemoteSelect
                  path={`/v1/tenants/${values.tenant_id}/database-targets?managed=true`}
                  label="Approved database target"
                  value={targetID}
                  initial={[]}
                  onChange={setTargetID}
                />
              </FormField>
              <Inline>
                <Button
                  size="sm"
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
                </Button>
              </Inline>
              {values.limits.target_ids.map((id) => (
                <Inline key={id}>
                  <Badge mono>{id}</Badge>
                  <IconButton
                    size="sm"
                    label={`Remove target ${id}`}
                    icon={<X size={14} aria-hidden />}
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
                  />
                </Inline>
              ))}
              <FormField label="Maximum new databases" required>
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
              </FormField>
            </FieldGroup>
          )}
          <FieldGroup
            framed
            legend="When does access end?"
            description="You can revoke access at any time. Closing the browser or signing out does not revoke this connection."
          >
            {quick.length > 0 && (
              <SegmentedControl
                label="Access length"
                options={quick.map((d) => ({
                  value: String(d),
                  label: `${d} days`,
                }))}
                value={String(values.days)}
                onChange={(d) => setValues({ ...values, days: Number(d) })}
              />
            )}
            <FormField
              label="Access expires on"
              hint={`${values.days} ${values.days === 1 ? "day" : "days"} from now; at most ${config.max_days}.`}
            >
              <DatePicker
                value={expiry}
                min={isoDay(addDays(today, 1))}
                max={isoDay(addDays(today, config.max_days))}
                onChange={(day) => {
                  const chosen = parseDay(day);
                  if (chosen)
                    setValues({ ...values, days: daysBetween(today, chosen) });
                }}
              />
            </FormField>
          </FieldGroup>
          {error && <Alert>{error}</Alert>}
        </Stack>
      </Body>
      <Footer>
        <Button onClick={cancel} disabled={save.busy}>
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          loading={save.busy}
          disabled={values.permissions.length === 0}
        >
          {label}
        </Button>
      </Footer>
    </form>
  );
}

export function AgentConsent({ me }: { me: Me }) {
  const [actionError, setError] = useState("");
  const request = new URLSearchParams(location.search).get("request") || "";
  const infoQuery = useApiQuery<AgentRequest>(
      `/v1/agent-access/requests/${encodeURIComponent(request)}`,
    ),
    configQuery = useApiQuery<AgentConfig>(agentConfigPath);
  const decline = useAction(request, {
    onSuccess: () => {},
    onError: setError,
  });
  const info = infoQuery.data,
    config = configQuery.data,
    error =
      actionError ||
      infoQuery.error?.message ||
      configQuery.error?.message ||
      "";
  function deny() {
    void decline.run(async () => {
      const result = await api<Redirect>(
        `/v1/agent-access/requests/${request}`,
        "POST",
        { deny: true },
      );
      location.assign(result.redirect_url);
    });
  }
  return (
    <StandalonePage>
      <PageHeader
        title="Connect an agent"
        description={`Choose exactly what this client may do as ${me.display_name}.`}
      />
      {error && <Alert>{error}</Alert>}
      {info && config ? (
        <Card className="agent-consent-card">
          <CardHeader
            title={
              <>
                <Avatar kind="agent" name={info.client_name} size={24} />{" "}
                {info.client_name}
              </>
            }
            description={`Returns to ${new URL(info.redirect_uri).host}`}
          />
          <AgentGrantForm
            config={config}
            me={me}
            intro={
              <>
                <FactList
                  facts={[
                    { label: "Client ID", value: info.client_id, mono: true },
                  ]}
                />
                <Alert tone="warning" live={false}>
                  Check this client identity before approving. Its display name
                  is provided by the client.
                </Alert>
                <Alert tone="info" live={false}>
                  Configuration management only. This connection cannot read or
                  change business records or retrieve credentials.
                </Alert>
              </>
            }
            initial={{
              ...initialGrant(config, me),
              permissions: config.default_permissions.filter((code) =>
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
        </Card>
      ) : (
        !error && (
          <Card>
            <CardBody>
              <Stack gap="sm">
                <Skeleton width="45%" />
                <Skeleton width="70%" />
                <Skeleton width="60%" />
              </Stack>
            </CardBody>
          </Card>
        )
      )}
    </StandalonePage>
  );
}

function AgentGrantDialog({
  grant,
  config,
  me,
  close,
  done,
}: {
  grant: AgentGrant;
  config: AgentConfig;
  me: Me;
  close: () => void;
  done: () => void;
}) {
  return (
    <Dialog
      title="Agent access"
      description={grant.client_name}
      titleId="agent-dialog-title"
      closeLabel="Close agent access"
      size="lg"
      closeOnBackdrop={false}
      onClose={close}
    >
      <AgentGrantForm
        config={config}
        me={me}
        initial={initialGrant(config, me, grant)}
        locked
        inDialog
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
    </Dialog>
  );
}

// One connection: the agent as an actor acting for its owner inside a scope.
function AgentConnection({
  grant,
  config,
  me,
  tenant,
  revoking,
  edit,
  revoke,
}: {
  grant: AgentGrant;
  config?: AgentConfig;
  me: Me;
  tenant?: string;
  revoking: boolean;
  edit: () => void;
  revoke: () => void;
}) {
  const active = grant.state === "active";
  return (
    <Card as="article" className="agent-connection">
      <CardHeader
        level={3}
        title={
          <>
            <Avatar kind="agent" name={grant.client_name} size={24} />{" "}
            {grant.client_name}
          </>
        }
        description={`For ${grant.owner_name} · ${grant.scope_name} · ${grant.scope_kind}`}
        actions={<StatusBadge state={grant.state} />}
      />
      <CardBody>
        <Stack gap="md">
          <Inline gap="sm">
            {grant.permissions_json.map((code) => (
              <Badge key={code}>
                {config?.permissions.find((p) => p.code === code)?.label ||
                  code}
              </Badge>
            ))}
          </Inline>
          <FactList
            facts={[
              ...(!tenant && grant.tenant_name
                ? [{ label: "Tenant", value: grant.tenant_name }]
                : []),
              { label: "Expires", value: when(grant.expires_at) },
              {
                label: "Last mutation",
                value: grant.last_used_at ? when(grant.last_used_at) : "None",
              },
            ]}
          />
        </Stack>
      </CardBody>
      {active && (
        <CardFooter>
          {grant.user_id === me.id && (
            <Button onClick={edit}>Edit access</Button>
          )}
          <Button variant="danger" loading={revoking} onClick={revoke}>
            Revoke access
          </Button>
        </CardFooter>
      )}
    </Card>
  );
}

export function AgentAccess({ me, tenant }: { me: Me; tenant?: string }) {
  const [page, setPage] = useState(1),
    [actionError, setError] = useState(""),
    [editing, setEditing] = useState<AgentGrant | null>(null),
    [revoking, setRevoking] = useState(""),
    [copied, setCopied] = useState(false);
  const configQuery = useApiQuery<AgentConfig>(agentConfigPath),
    listQuery = useApiQuery<Page<AgentGrant>>(
      `/v1/agent-access?page=${page}&limit=20${tenant ? "&tenant_id=" + tenant : ""}`,
    );
  const revoke = useAction(page, { onSuccess: () => {}, onError: setError });
  const config = configQuery.data,
    loading = configQuery.isPending || listQuery.isPending,
    loadFailed = !!(configQuery.error || listQuery.error),
    items = listQuery.data?.items || [],
    more = !!listQuery.data?.pagination?.has_more,
    error =
      actionError ||
      configQuery.error?.message ||
      listQuery.error?.message ||
      "";
  function load() {
    setError("");
    void configQuery.refetch();
    void listQuery.refetch();
  }
  async function confirmRevoke(grant: AgentGrant) {
    if (
      !(await confirmAction({
        title: "Revoke agent access?",
        message: `${grant.client_name} stops acting for ${grant.owner_name} in ${grant.scope_name} at once. A revoked connection cannot be restored; the agent must connect again.`,
        confirmLabel: "Revoke access",
        danger: true,
      }))
    )
      return;
    setRevoking(grant.id);
    void revoke.run(() =>
      api(
        `/v1/agent-access/${grant.id}`,
        "PATCH",
        { state: "revoked" },
        grant.revision,
      ),
    );
  }
  return (
    <Stack gap="md" className="agent-access">
      <Card>
        <CardHeader
          title="Agent access"
          description={
            tenant
              ? "Review connections approved by members of this Tenant and revoke access when needed."
              : "Review the clients acting on your behalf and revoke them when finished."
          }
          actions={
            <Button
              icon={<RefreshCw size={15} aria-hidden />}
              disabled={configQuery.isFetching || listQuery.isFetching}
              onClick={load}
            >
              Refresh connections
            </Button>
          }
        />
        {config && (
          <CardBody>
            <Stack gap="sm">
              <Inline gap="md">
                <strong>Connect from Codex or Claude</strong>
                <Button
                  size="sm"
                  icon={<Copy size={14} aria-hidden />}
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
                </Button>
              </Inline>
              <CodeBlock label="MCP URL">{config.mcp_url}</CodeBlock>
              <small>
                Add this URL in your agent client. It opens Lastapp sign-in and
                asks you to approve access. No API keys or OAuth client setup
                required.
              </small>
            </Stack>
          </CardBody>
        )}
      </Card>
      {error && <Alert>{error}</Alert>}
      {loading ? (
        <Card>
          <CardBody>
            <Stack gap="sm">
              <Skeleton width="40%" />
              <Skeleton width="65%" />
            </Stack>
          </CardBody>
        </Card>
      ) : loadFailed ? null : items.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Bot size={28} aria-hidden />}
            title="No agent connections yet"
          >
            Add the MCP endpoint to your agent client to begin.
          </EmptyState>
        </Card>
      ) : (
        <Columns min={340} gap="md">
          {items.map((grant) => (
            <AgentConnection
              key={grant.id}
              grant={grant}
              config={config}
              me={me}
              tenant={tenant}
              revoking={revoke.busy && revoking === grant.id}
              edit={() => setEditing(grant)}
              revoke={() => void confirmRevoke(grant)}
            />
          ))}
        </Columns>
      )}
      {(page > 1 || more) && (
        <Inline>
          <Button disabled={page === 1} onClick={() => setPage(page - 1)}>
            Previous
          </Button>
          <span>Page {page}</span>
          <Button disabled={!more} onClick={() => setPage(page + 1)}>
            Next
          </Button>
        </Inline>
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
            void refreshQueries();
          }}
        />
      )}
    </Stack>
  );
}
