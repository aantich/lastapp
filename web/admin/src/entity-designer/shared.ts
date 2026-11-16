import type { Dispatch, SetStateAction } from "react";
import type { EntityCatalog, Item } from "../api-types";
import { confirmAction } from "../confirm";

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
