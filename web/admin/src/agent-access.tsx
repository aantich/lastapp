import { useState, type ReactNode } from "react";
import { Bot, RefreshCw, X } from "lucide-react";
import { api } from "./api";
import {
  addDays,
  Alert,
  Badge,
  Button,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  ChoiceCards,
  Columns,
  ConnectionCard,
  ConnectionGrid,
  confirmAction,
  DatePicker,
  daysBetween,
  Dialog,
  DialogBody,
  DialogFooter,
  Disclosure,
  EmptyState,
  FactList,
  FlowCard,
  FlowSection,
  FlowStack,
  FormField,
  IconButton,
  Inline,
  isoDay,
  NumberInput,
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
import { agentConfigPath, McpConnection } from "./connect-agent";
import { refreshQueries, useApiQuery } from "./query";
import { ago } from "./actors";

import type {
  AgentConfig,
  AgentGrant,
  AgentLimits,
  AgentRequest,
  Me,
  Page,
  Redirect,
} from "./api-types";
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
const dayLabel = (d: number) => `${d} ${d === 1 ? "day" : "days"}`;
const kindLabel = (kind: string) =>
  kind ? kind[0].toUpperCase() + kind.slice(1) : "";

// The preset whose permissions are exactly these, if any. A grant stores
// permissions, never a preset, so this is a reading of them.
const presetOf = (config: AgentConfig | undefined, codes: string[]) =>
  config?.presets.find((p) => sameSet(p.permissions, codes));

// The scope → preset → permissions → limits → expiry steps shared by the
// consent page and the edit dialog. The backend contract stays a number of
// days; the quick choices come from the config's day_choices and the date
// picker is a view of the same number, bounded by max_days.
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
  title,
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
  // The consent page's card title; the dialog has its own.
  title?: string;
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
  // A preset is offered only when every permission it names may be granted
  // here (the client requested it and the scope kind allows it).
  const grantable = (codes: string[]) =>
    codes.every((code) => permissionList.some((p) => p.code === code));
  const provisioning = values.permissions.includes("databases.provision");
  const scopePath =
    values.scope_kind === "space"
      ? "spaces"
      : values.scope_kind === "app"
        ? "apps"
        : "teams";
  const today = new Date(),
    expiry = isoDay(addDays(today, values.days)),
    quick = (config.day_choices || []).filter(
      (d) => d >= 1 && d <= config.max_days,
    );
  const preset = presetOf(config, values.permissions);
  function permissions(codes: string[]) {
    setValues({
      ...values,
      permissions: codes.filter((code) =>
        permissionList.some((p) => p.code === code),
      ),
    });
  }
  const sections = (
    <>
      {intro}
      {locked ? (
        <Alert tone="info" live={false}>
          Bound to {values.scope_kind}
          {values.scope_id ? ` · ${values.scope_id}` : ""}. Reconnect to choose
          a different scope.
        </Alert>
      ) : (
        <FlowSection label="Where may it work?">
          <Columns min={200} gap="md">
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
                      : kindLabel(kind),
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
        </FlowSection>
      )}
      <FlowSection label="What may it do?">
        <ChoiceCards
          label="Permission presets"
          options={config.presets.map((p) => ({
            value: p.label,
            label: p.label,
            description: p.description,
            disabled: !grantable(p.permissions),
          }))}
          value={preset?.label || ""}
          onChange={(label) =>
            permissions(
              config.presets.find((p) => p.label === label)?.permissions || [],
            )
          }
        />
        <Inline gap="sm">
          {values.permissions.map((code) => (
            <Badge key={code} pill mono>
              {code}
            </Badge>
          ))}
        </Inline>
        <Disclosure
          summary="Choose individual permissions"
          meta={
            preset
              ? "Presets only tick boxes"
              : `${values.permissions.length} checked`
          }
          defaultOpen={!preset}
        >
          <Stack gap="sm">
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
          </Stack>
        </Disclosure>
      </FlowSection>
      {provisioning && (
        <FlowSection label="Database limits">
          <small>
            Only these targets may receive new databases. The count includes
            allocations made during Space creation.
          </small>
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
        </FlowSection>
      )}
      <FlowSection label="Expires after">
        {quick.length > 0 && (
          <SegmentedControl
            label="Access length"
            options={quick.map((d) => ({
              value: String(d),
              label: dayLabel(d),
            }))}
            value={String(values.days)}
            onChange={(d) => setValues({ ...values, days: Number(d) })}
          />
        )}
        <FormField
          label="Access expires on"
          hint={`${dayLabel(values.days)} from now; at most ${config.max_days}. Closing the browser or signing out does not end it.`}
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
      </FlowSection>
      <p>
        It never gets business records, database credentials or SQL. You can
        revoke it any time.
      </p>
      {error && <Alert>{error}</Alert>}
    </>
  );
  const buttons = (
    <>
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
    </>
  );
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void save.run(() => submit(values));
      }}
    >
      {inDialog ? (
        <>
          <DialogBody>
            <FlowStack className="agent-grant-form">{sections}</FlowStack>
          </DialogBody>
          <DialogFooter>{buttons}</DialogFooter>
        </>
      ) : (
        <FlowCard
          title={title}
          titleId="agent-consent-title"
          level={1}
          className="agent-consent-card agent-grant-form"
          footer={buttons}
        >
          {sections}
        </FlowCard>
      )}
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
    <StandalonePage width={730}>
      {error && <Alert>{error}</Alert>}
      {info && config ? (
        <AgentGrantForm
          config={config}
          me={me}
          title="Connect an agent"
          intro={
            <FlowSection label="Agent">
              <Inline gap="sm">
                <strong>{info.client_name}</strong>
                <span>Returns to {new URL(info.redirect_uri).host}</span>
              </Inline>
              <FactList
                facts={[
                  { label: "Client ID", value: info.client_id, mono: true },
                ]}
              />
              <Alert tone="warning" live={false}>
                {info.client_name} is the name the client gives itself; it is
                not verified. Check the client ID and return address before
                approving. You act as {me.display_name}.
              </Alert>
            </FlowSection>
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
          label="Approve connection"
        />
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

// One connection: the agent (its claimed name) acting for its owner inside
// a scope, the preset its permissions match, the permissions themselves,
// expiry and last use. `now` is when the grants were read.
function AgentConnection({
  grant,
  config,
  me,
  tenant,
  now,
  revoking,
  edit,
  revoke,
}: {
  grant: AgentGrant;
  config?: AgentConfig;
  me: Me;
  tenant?: string;
  now: number;
  revoking: boolean;
  edit: () => void;
  revoke: () => void;
}) {
  const expired =
    grant.state === "active" && new Date(grant.expires_at).getTime() <= now;
  const active = grant.state === "active" && !expired;
  const preset = presetOf(config, grant.permissions_json);
  const scope =
    grant.scope_kind === "tenant"
      ? `Tenant ${grant.tenant_name || grant.scope_name}`
      : `${kindLabel(grant.scope_kind)} ${grant.scope_name}`;
  return (
    <ConnectionCard
      className="agent-connection"
      name={grant.client_name}
      subtitle={
        `for ${grant.owner_name} · ${scope}` +
        (!tenant && grant.tenant_name && grant.scope_kind !== "tenant"
          ? ` · ${grant.tenant_name}`
          : "")
      }
      status={
        expired ? (
          <StatusBadge state="expired" />
        ) : (
          <StatusBadge state={grant.state} />
        )
      }
      inactive={!active}
      chips={
        <>
          {config && (
            <Badge pill tone="info">
              {preset?.label || "Custom"}
            </Badge>
          )}
          {grant.permissions_json.map((code) => (
            <Badge
              key={code}
              pill
              mono
              title={config?.permissions.find((p) => p.code === code)?.label}
            >
              {code}
            </Badge>
          ))}
          {grant.permissions_json.includes("databases.provision") && (
            <Badge pill>
              At most {grant.limits_json.max_databases} new databases on{" "}
              {grant.limits_json.target_ids.length} approved targets
            </Badge>
          )}
        </>
      }
      meta={
        <>
          <span>
            {expired ? "Expired " : "Expires "}
            {ago(grant.expires_at)}
          </span>
          <span aria-hidden>·</span>
          <span>
            {grant.last_used_at
              ? `last used ${ago(grant.last_used_at)}`
              : "never used"}
          </span>
        </>
      }
      actions={
        active && (
          <>
            {grant.user_id === me.id && (
              <Button size="sm" variant="ghost" onClick={edit}>
                Edit
              </Button>
            )}
            <Button
              size="sm"
              variant="danger"
              loading={revoking}
              onClick={revoke}
            >
              Revoke
            </Button>
          </>
        )
      }
    />
  );
}

// Agent connections: a Tenant's (for a Tenant security manager, `tenant`)
// or the person's own. With `heading` the list sits under an "Agent access"
// card with the MCP URL (account and Tenant settings); the Agents page
// draws its own header instead.
export function AgentAccess({
  me,
  tenant,
  heading = true,
}: {
  me: Me;
  tenant?: string;
  heading?: boolean;
}) {
  const [page, setPage] = useState(1),
    [actionError, setError] = useState(""),
    [editing, setEditing] = useState<AgentGrant | null>(null),
    [revoking, setRevoking] = useState("");
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
      {heading && (
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
              <McpConnection config={config} />
            </CardBody>
          )}
        </Card>
      )}
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
        <ConnectionGrid>
          {items.map((grant) => (
            <AgentConnection
              key={grant.id}
              grant={grant}
              config={config}
              me={me}
              tenant={tenant}
              now={listQuery.dataUpdatedAt}
              revoking={revoke.busy && revoking === grant.id}
              edit={() => setEditing(grant)}
              revoke={() => void confirmRevoke(grant)}
            />
          ))}
        </ConnectionGrid>
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
