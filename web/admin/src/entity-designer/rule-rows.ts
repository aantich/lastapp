import { items as itemsOf, text } from "../api-types";
import { describe, type Designer } from "./shared";

export type RuleRow = {
  key: string;
  title: string;
  kind: string;
  expression: string;
  note: string;
  edit: () => void;
  remove?: () => void;
};

// The Rules tab's rows: fields with a default or calculation, then the
// validation and allowed-change rules.
export function ruleRows(d: Designer): RuleRow[] {
  const { draft, remove, setEditField, setEditRule } = d;
  const fields = itemsOf(draft.fields);
  return [
    ...fields
      .filter((f) => f.default || f.calculation)
      .map((f) => ({
        key: "field:" + text(f.key),
        title: text(f.label),
        kind: f.calculation ? "calculation" : "default",
        expression:
          text(f.label) + " = " + describe(f.calculation || f.default, fields),
        note: f.calculation ? "stored" : "on create",
        edit: () => setEditField({ ...f }),
      })),
    ...itemsOf(draft.rules).map((rule) => ({
      key: "rule:" + text(rule.key),
      title: text(rule.name || rule.key),
      kind: rule.kind === "change" ? "allowed change" : "validation",
      expression:
        (rule.when ? "when " + describe(rule.when, fields) + ": " : "") +
        describe(rule.assert, fields),
      note: rule.message
        ? "“" + text(rule.message) + "”"
        : rule.kind === "change"
          ? "on update"
          : "on save",
      edit: () => setEditRule({ ...rule }),
      remove: () => remove("rules", text(rule.key)),
    })),
  ];
}
