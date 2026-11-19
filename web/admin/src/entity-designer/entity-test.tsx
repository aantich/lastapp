import { useState } from "react";
import { api } from "../api";
import {
  Alert,
  Badge,
  Button,
  CodeBlock,
  Columns,
  DataTable,
  DesignerNote,
  Disclosure,
  FieldGroup,
  FormField,
  Inline,
  Panel,
  RemoteSelect,
  ResultTile,
  ResultTiles,
  Select,
  Stack,
} from "../ui";
import { isItem, items, text, type Item } from "../api-types";
import { strings } from "./shared";
import { ValueInputs } from "./value-inputs";

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
    <Panel
      title="Test as user"
      lead="Runs the real permission engine on one synthetic record. Nothing is read or written."
    >
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
        <Stack gap="md">
          {error && <Alert as="p">{error}</Alert>}
          <Columns min="sm" gap="md">
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
                options={["create", "read", "update", "delete"].map((v) => ({
                  value: v,
                  label: v,
                }))}
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
          <Inline>
            <Button type="submit" variant="primary" loading={running}>
              Run test
            </Button>
          </Inline>
        </Stack>
      </form>
      {result && <TestResult result={result} />}
    </Panel>
  );
}

// The outcome: allowed or denied, the fields the complete matching paths
// reach, and why, all from the evaluator's own answer.
function TestResult({ result }: { result: Item }) {
  const paths = items(result.permission_paths),
    matching = paths.filter((p) => p.matches_complete_path),
    reached = [...new Set(matching.flatMap((p) => strings(p.fields)))],
    notChecked = strings(result.not_checked);
  const why = result.allowed
    ? matching.length
      ? "Allowed through " + matching.map((p) => text(p.label)).join(", ") + "."
      : "Allowed."
    : text(result.error) || "No permission path matches this scenario.";
  return (
    <Stack gap="md">
      <div aria-live="polite">
        <ResultTiles>
          <ResultTile label="Result">
            {result.allowed ? "Allowed" : "Denied"}
          </ResultTile>
          <ResultTile label="Fields" size="mono" grow={2}>
            {reached.join(", ") || "—"}
          </ResultTile>
        </ResultTiles>
      </div>
      <DesignerNote>{why}</DesignerNote>
      {notChecked.length > 0 && (
        <DesignerNote>Not checked: {notChecked.join(", ")}.</DesignerNote>
      )}
      {items(result.diagnostics).map((d, i) => (
        <Alert as="p" tone="warning" live={false} key={i}>
          {text(d.path)}: {text(d.message)}
        </Alert>
      ))}
      {paths.length > 0 && (
        <Disclosure summary="Permission paths in this scenario">
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
        </Disclosure>
      )}
      {!!result.candidate && (
        <Disclosure summary="Candidate record">
          <CodeBlock label="Candidate record">
            {JSON.stringify(result.candidate, null, 2)}
          </CodeBlock>
        </Disclosure>
      )}
    </Stack>
  );
}
