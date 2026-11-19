import { X } from "lucide-react";
import {
  Badge,
  Button,
  FlowSection,
  FormField,
  IconButton,
  Inline,
  NumberInput,
  RemoteSelect,
} from "../ui";
import type { AgentConfig } from "../api-types";
import type { GrantValuesProps } from "./grant-values";

// The approved database targets and count for an agent that provisions
// databases. `targetID` is the target chosen but not yet approved.
export function GrantDatabaseLimits({
  config,
  values,
  setValues,
  targetID,
  setTargetID,
}: GrantValuesProps & {
  config: AgentConfig;
  targetID: string;
  setTargetID: (id: string) => void;
}) {
  return (
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
                  target_ids: values.limits.target_ids.filter((v) => v !== id),
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
  );
}
