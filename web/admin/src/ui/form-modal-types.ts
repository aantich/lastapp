import type { Item } from "../api-types";

export type Field = {
  key: string;
  label: string;
  type?: string;
  options?: Item[];
  required?: boolean;
  newPassword?: boolean;
  hint?: string;
  lookup?: string;
  value?: unknown;
  when?: (values: Item) => boolean;
  generate?: boolean;
  advanced?: boolean;
  excludeControl?: boolean;
  excludePlatform?: boolean;
  excludeInactive?: boolean;
  wide?: boolean;
};
export type Modal = {
  title: string;
  description?: string;
  fields: Field[];
  initial: Item;
  save: (v: Item) => Promise<unknown>;
  submit?: string;
  // The confirmation toast after a save; by default derived from the title
  // ("Create App" → "App created", "Edit team" → "Team saved").
  success?: string;
};
