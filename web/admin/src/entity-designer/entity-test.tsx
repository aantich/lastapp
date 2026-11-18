import { useState } from "react";
import { api } from "../api";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  CodeBlock,
  Columns,
  DataTable,
  FactList,
  FieldGroup,
  FormField,
  RemoteSelect,
  Select,
  Stack,
  StatusBadge,
} from "../ui";
import { isItem, items, text, type Item } from "../api-types";
import { strings } from "./shared";
import { ValueInputs } from "./expression";

// Runs the backend evaluator against the draft with synthetic values.
export function EntityTest({
  draft,
  record,
  tenant,
  space,
  me,
}: {
  draft: Item;
  record: Item;
  tenant: string;
  space: string;
  me: Item;
}) {
  const [scenario, set] = useState<Item>({
      user_id: me.id,
      app_id: "",
      operation: "create",
      record: {},
      values: {},
    }),
    [result, setResult] = useState<Item | null>(null),
    [error, setError] = useState(""),
    [running, setRunning] = useState(false);
  const fields = items(draft.fields);
  return (
    <Stack>
      <Card>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            setError("");
            setRunning(true);
            api<Item>("/v1/entities/" + record.id + "/test", "POST", {
              definition: draft,
              ...scenario,
            })
              .then(setResult)
              .catch((e) => setError(e.message))
              .finally(() => setRunning(false));
          }}
        >
          <CardHeader
            level={3}
            title="Try a synthetic scenario"
            description="This uses the backend evaluator with sample values against the current draft. It never reads or writes business records."
          />
          <CardBody>
            <Stack gap="md">
              {error && <Alert as="p">{error}</Alert>}
              <Columns min={220} gap="md">
                <FormField label="Test as user">
                  <RemoteSelect
                    initial={[]}
                    path={"/v1/tenants/" + tenant + "/recipients"}
                    label="Test as user"
                    value={text(scenario.user_id)}
                    onChange={(v) => set({ ...scenario, user_id: v })}
                  />
                </FormField>
                <FormField label="App context (optional)">
                  <RemoteSelect
                    initial={[]}
                    path={"/v1/tenants/" + tenant + "/apps?space_id=" + space}
                    label="App context"
                    value={text(scenario.app_id)}
                    onChange={(v) => set({ ...scenario, app_id: v })}
                  />
                </FormField>
                <FormField label="Operation">
                  <Select
                    value={text(scenario.operation)}
                    onChange={(operation) => set({ ...scenario, operation })}
                    options={["create", "read", "update", "delete"].map(
                      (v) => ({ value: v, label: v }),
                    )}
                  />
                </FormField>
              </Columns>
              {scenario.operation !== "create" && (
                <FieldGroup legend="Existing sample record">
                  <ValueInputs
                    fields={fields}
                    value={isItem(scenario.record) ? scenario.record : {}}
                    onChange={(v) => set({ ...scenario, record: v })}
                  />
                </FieldGroup>
              )}
              {["create", "update"].includes(text(scenario.operation)) && (
                <FieldGroup legend="Proposed values">
                  <ValueInputs
                    fields={fields}
                    value={isItem(scenario.values) ? scenario.values : {}}
                    onChange={(v) => set({ ...scenario, values: v })}
                  />
                </FieldGroup>
              )}
            </Stack>
          </CardBody>
          <CardFooter>
            <Button type="submit" variant="primary" loading={running}>
              Run test
            </Button>
          </CardFooter>
        </form>
      </Card>
      {result && <TestResult result={result} />}
    </Stack>
  );
}

function TestResult({ result }: { result: Item }) {
  const paths = items(result.permission_paths);
  return (
    <Card aria-live="polite">
      <CardHeader
        level={3}
        title={result.allowed ? "Allowed" : "Denied or invalid"}
        actions={
          <StatusBadge
            state={result.allowed ? "allowed" : "denied"}
            tone={result.allowed ? "success" : "danger"}
          />
        }
      />
      <CardBody>
        <Stack gap="md">
          {!!result.error && (
            <Alert as="p" live={false}>
              {text(result.error)}
            </Alert>
          )}
          {items(result.diagnostics).map((d, i) => (
            <Alert as="p" tone="warning" live={false} key={i}>
              {text(d.path)}: {text(d.message)}
            </Alert>
          ))}
          <FactList
            facts={[
              {
                label: "Not checked",
                value: strings(result.not_checked).join(", ") || "—",
              },
            ]}
          />
          {paths.length > 0 && (
            <DataTable
              caption="Permission paths in this scenario"
              rows={paths}
              rowKey={(p) => text(p.key)}
              columns={[
                {
                  key: "path",
                  header: "Path",
                  render: (p) => <strong>{text(p.label)}</strong>,
                },
                {
                  key: "fields",
                  header: "Fields",
                  render: (p) => strings(p.fields).join(", ") || "Record",
                },
                {
                  key: "match",
                  header: "Complete path",
                  render: (p) =>
                    p.matches_complete_path ? (
                      <Badge tone="success">Matches</Badge>
                    ) : (
                      <Badge>No match</Badge>
                    ),
                },
              ]}
            />
          )}
          {!!result.candidate && (
            <CodeBlock label="Candidate record">
              {JSON.stringify(result.candidate, null, 2)}
            </CodeBlock>
          )}
        </Stack>
      </CardBody>
    </Card>
  );
}
