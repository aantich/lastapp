import { useState, type ReactNode } from "react";
import {
  Alert,
  Button,
  DialogBody,
  DialogFooter,
  FlowCard,
  FlowStack,
} from "../ui";
import type { AgentConfig, Me } from "../api-types";
import { useAction } from "../use-action";
import type { GrantValues } from "./grant-values";
import { GrantScope } from "./grant-scope";
import { GrantPermissions } from "./grant-permissions";
import { GrantDatabaseLimits } from "./grant-database-limits";
import { GrantExpiry } from "./grant-expiry";

// The scope → preset → permissions → limits → expiry steps shared by the
// consent page and the edit dialog. The backend contract stays a number of
// days; the quick choices come from the config's day_choices and the date
// picker is a view of the same number, bounded by max_days.
export function AgentGrantForm({
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
  const provisioning = values.permissions.includes("databases.provision");
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
        <GrantScope
          me={me}
          values={values}
          setValues={setValues}
          eligiblePermissions={eligiblePermissions}
        />
      )}
      <GrantPermissions
        config={config}
        values={values}
        setValues={setValues}
        permissionList={permissionList}
      />
      {provisioning && (
        <GrantDatabaseLimits
          config={config}
          values={values}
          setValues={setValues}
          targetID={targetID}
          setTargetID={setTargetID}
        />
      )}
      <GrantExpiry config={config} values={values} setValues={setValues} />
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
