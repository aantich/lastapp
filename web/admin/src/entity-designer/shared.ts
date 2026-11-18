import type { Dispatch, SetStateAction } from "react";
import {
  isItem,
  items as itemsOf,
  text,
  type EntityCatalog,
  type Item,
} from "../api-types";
import { confirmAction } from "../ui";
import type { Expression } from "../../../../packages/client/src/index.ts";

export const keyOf = (name: string) => {
  const bare = name
    .normalize("NFKD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return (
    bare
      ? /^[a-z]/.test(bare)
        ? bare
        : "item_" + bare
      : "item_" + crypto.randomUUID().replaceAll("-", "")
  ).slice(0, 48);
};
// A choice value's display label; keys without one show as themselves.
// Only the labels object's own keys count, never inherited properties.
export const choiceLabel = (field: Item, key: string): string => {
  const labels = isItem(field.value_labels) ? field.value_labels : {},
    label = Object.hasOwn(labels, key) ? labels[key] : undefined;
  return typeof label === "string" ? label : key;
};
// Narrowing for the loose definition documents the designer edits. They are
// written by the backend's entity model (internal/entity/model.go), so these
// only give the checker the shape the values already have.

// An entity record's draft_json or active_definition: the definition object,
// or null when there is none.
export const definitionOf = (v: unknown): Item | null => (isItem(v) ? v : null);
// A list of strings (field keys, role IDs, choice values); [] when absent.
export const strings = (v: unknown): string[] =>
  Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
// A loose value as a controlled input's value: strings and numbers as they
// are, anything else unset (undefined), as React treats null or undefined.
export const inputValue = (v: unknown): string | number | undefined =>
  typeof v === "string" || typeof v === "number" ? v : undefined;
// A loose value that is a string, else undefined (a Select's value).
export const optionalText = (v: unknown): string | undefined =>
  typeof v === "string" ? v : undefined;
// An expression document (a condition, formula or default). The backend
// validates its structure; the editor only needs it to be an object.
export const isExpression = (v: unknown): v is Expression => isItem(v);
export const fieldLabel = (key: string) => key[0].toUpperCase() + key.slice(1);

// How the designer names a permission's relationship ("Who").
export const relationshipLabels: Record<string, string> = {
  member: "All active Tenant members",
  owner: "Record owner",
  team_member: "Members of record's team",
  team_reviewer: "Reviewers of record's team",
  team_admin: "Admins of record's team",
  user: "Specific user",
  team: "Specific team",
};
// Who a permission path is for: its Space roles by name, or its relationship.
export const whoOf = (p: Item, roles: Item[]) =>
  strings(p.roles)
    .map((id) => text(roles.find((r) => r.id === id)?.display_name) || id)
    .join(", ") ||
  relationshipLabels[text(p.relationship)] ||
  text(p.relationship) ||
  "No one yet";

// An entity's lifecycle as the list and header show it.
export const entityState = (v: Item) =>
  v.pending_version_id
    ? "publishing"
    : v.state === "archived"
      ? "archived"
      : v.active_version_id
        ? "published"
        : "draft";

// A version's number as "v8", or "" when there is none. The backend assigns
// the numbers (version_number, active_/pending_/next_version_number).
export const versionLabel = (v: unknown): string =>
  typeof v === "number" && v > 0 ? "v" + v : "";

// A definition value with its empty members dropped and keys sorted, so a
// draft edited in the browser compares equal to the backend's encoding of
// the same definition (which omits empty values).
const canonical = (v: unknown): unknown => {
  if (Array.isArray(v)) return v.map(canonical);
  if (!isItem(v)) return v;
  const out: Item = {};
  for (const key of Object.keys(v).sort()) {
    const x = canonical(v[key]);
    const empty =
      x === undefined ||
      x === null ||
      x === "" ||
      x === false ||
      (Array.isArray(x) && x.length === 0) ||
      (isItem(x) && Object.keys(x).length === 0);
    if (!empty) out[key] = x;
  }
  return out;
};
const same = (a: unknown, b: unknown) =>
  JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));
const keyed = (v: unknown) =>
  new Map(
    (Array.isArray(v) ? v : []).filter(isItem).map((x) => [text(x.key), x]),
  );

// How many changes the draft makes to the active definition: each field,
// rule and permission added, removed or changed by its key, and each
// changed entity setting. Null when nothing is published.
export function draftChanges(draft: Item, active: unknown): number | null {
  if (!isItem(active)) return null;
  let n = 0;
  for (const list of ["fields", "rules", "permissions"]) {
    const before = keyed(active[list]),
      after = keyed(draft[list]);
    for (const [key, v] of after) if (!same(v, before.get(key))) n++;
    for (const key of before.keys()) if (!after.has(key)) n++;
  }
  for (const key of new Set([...Object.keys(draft), ...Object.keys(active)]))
    if (
      !["fields", "rules", "permissions"].includes(key) &&
      !same(draft[key], active[key])
    )
      n++;
  return n;
}

// A one-line reading of an expression for lists ("Stage ≠ "lost""). The
// editor and the backend work on the expression itself; this is display only.
const infix: Record<string, string> = {
  eq: "=",
  ne: "≠",
  lt: "<",
  lte: "≤",
  gt: ">",
  gte: "≥",
  add: "+",
  sub: "−",
  mul: "×",
  div: "÷",
};
export function describe(x: unknown, fields: Item[], depth = 0): string {
  if (!isExpression(x)) return "";
  const label = (key: string) => {
    const [prefix, bare] = key.includes(".") ? key.split(".", 2) : ["", key];
    const name = text(fields.find((f) => f.key === bare)?.label) || bare;
    return prefix === "old"
      ? "previous " + name
      : prefix === "new"
        ? "resulting " + name
        : name;
  };
  if (x.op) {
    const args = (x.args || []).map((a) => describe(a, fields, depth + 1));
    const op = String(x.op);
    const out = infix[op]
      ? args.join(" " + infix[op] + " ")
      : op === "all" || op === "any"
        ? args.join(op === "all" ? " and " : " or ")
        : op === "not"
          ? "not " + args[0]
          : op === "is_null"
            ? args[0] + " is empty"
            : op === "is_not_null"
              ? args[0] + " is set"
              : op === "in"
                ? args[0] + " in (" + args.slice(1).join(", ") + ")"
                : op + "(" + args.join(", ") + ")";
    return depth > 0 && args.length > 1 && op !== "coalesce" && op !== "round"
      ? "(" + out + ")"
      : out;
  }
  if (x.field) return label(String(x.field));
  if (x.context) return String(x.context);
  if (x.decimal !== undefined) return String(x.decimal);
  return x.literal === undefined ? "" : JSON.stringify(x.literal);
}
export const discardChanges = () =>
  confirmAction({
    title: "Discard unsaved changes?",
    message: "Your unsaved entity changes will be lost.",
    confirmLabel: "Discard changes",
    danger: true,
  });
export const emptyDefinition = (name: string, format: number) => ({
  format_version: format,
  key: keyOf(name),
  label: name,
  fields: [],
  rules: [],
  permissions: [],
  default_projection: [],
});

// What the designer's panels read from EntityDesigner: the open record and
// its draft, the catalog and roles, the editor dialogs and the shared action
// runner. EntityDesigner owns this state; panels only render and edit it.
export type Designer = {
  space: string;
  tenant: string;
  me: Item;
  catalog: EntityCatalog;
  roles: Item[];
  // The Space's entities (the list), for reference targets' names.
  entities: Item[];
  record: Item | null;
  setRecord: Dispatch<SetStateAction<Item | null>>;
  draft: Item;
  setDraft: Dispatch<SetStateAction<Item | null>>;
  dirty: boolean;
  busy: boolean;
  run: (fn: () => Promise<unknown>) => Promise<void>;
  save: () => Promise<Item>;
  remove: (list: string, key: string) => Promise<void>;
  setError: (message: string) => void;
  setReview: (open: boolean) => void;
  setEditField: (field: Item | null) => void;
  setEditPermission: (permission: Item | null) => void;
  setEditRule: (rule: Item | null) => void;
};

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
