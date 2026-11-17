import type { Dispatch, SetStateAction } from "react";
import { isItem, type EntityCatalog, type Item } from "../api-types";
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
