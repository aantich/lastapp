import { useState } from "react";
import { api } from "../api";
import { RemoteSelect, Select, Alert } from "../ui";
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
    [error, setError] = useState("");
  return (
    <form
      className="panel entity-test"
      onSubmit={(e) => {
        e.preventDefault();
        setError("");
        api<Item>("/v1/entities/" + record.id + "/test", "POST", {
          definition: draft,
          ...scenario,
        })
          .then(setResult)
          .catch((e) => setError(e.message));
      }}
    >
      <h3>Try a synthetic scenario</h3>
      <p>
        This uses the backend evaluator with sample values. It never reads or
        writes business records.
      </p>
      <label className="field">
        <span>Test as user</span>
        <RemoteSelect
          initial={[]}
          path={"/v1/tenants/" + tenant + "/recipients"}
          label="Test as user"
          value={text(scenario.user_id)}
          onChange={(v) => set({ ...scenario, user_id: v })}
        />
      </label>
      <label className="field">
        <span>App context (optional)</span>
        <RemoteSelect
          initial={[]}
          path={"/v1/tenants/" + tenant + "/apps?space_id=" + space}
          label="App context"
          value={text(scenario.app_id)}
          onChange={(v) => set({ ...scenario, app_id: v })}
        />
      </label>
      <label className="field">
        <span>Operation</span>
        <Select
          value={text(scenario.operation)}
          onChange={(operation) => set({ ...scenario, operation })}
          options={["create", "read", "update", "delete"].map((v) => ({
            value: v,
            label: v,
          }))}
        />
      </label>
      {scenario.operation !== "create" && (
        <>
          <h4>Existing sample record</h4>
          <ValueInputs
            fields={items(draft.fields)}
            value={isItem(scenario.record) ? scenario.record : {}}
            onChange={(v) => set({ ...scenario, record: v })}
          />
        </>
      )}
      {["create", "update"].includes(text(scenario.operation)) && (
        <>
          <h4>Proposed values</h4>
          <ValueInputs
            fields={items(draft.fields)}
            value={isItem(scenario.values) ? scenario.values : {}}
            onChange={(v) => set({ ...scenario, values: v })}
          />
        </>
      )}
      <button className="primary">Run test</button>
      {error && <Alert as="p">{error}</Alert>}
      {result && (
        <section className="entity-result">
          <h3>{result.allowed ? "Allowed" : "Denied or invalid"}</h3>
          {!!result.error && <p>{text(result.error)}</p>}
          {items(result.diagnostics).map((d, i) => (
            <p key={i}>
              {text(d.path)}: {text(d.message)}
            </p>
          ))}
          <p>Not checked: {strings(result.not_checked).join(", ")}</p>
          {items(result.permission_paths).map((p) => (
            <p key={text(p.key)}>
              {p.matches_complete_path ? "✓" : "–"} {text(p.label)} ·{" "}
              {strings(p.fields).join(", ") || "Record"}
            </p>
          ))}
          {!!result.candidate && (
            <pre>{JSON.stringify(result.candidate, null, 2)}</pre>
          )}
        </section>
      )}
    </form>
  );
}
