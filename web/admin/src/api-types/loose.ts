// Loose JSON values. The backend writes most responses as untyped Go maps
// (control.Row); these narrow their values where a screen reads them.

// A registry row or form value whose columns vary by endpoint. Its values are
// unknown: narrow them where they are used (the helpers below, typeof checks)
// or prefer a named shape below when a screen depends on specific fields.
export type Item = Record<string, unknown>;

// Display text of a loose value, as React would render it: strings and
// numbers as written, nothing for null, undefined, booleans or objects.
export const text = (v: unknown): string =>
  typeof v === "string" ? v : typeof v === "number" ? String(v) : "";

// A loose value that is a number, or undefined.
export const num = (v: unknown): number | undefined =>
  typeof v === "number" ? v : undefined;

// A loose value that is a JSON object.
export const isItem = (v: unknown): v is Item =>
  typeof v === "object" && v !== null && !Array.isArray(v);

// A loose value that is an array of JSON objects (other entries dropped).
export const items = (v: unknown): Item[] =>
  Array.isArray(v) ? v.filter(isItem) : [];

// A loose value usable as a React key: strings and numbers as written,
// undefined otherwise (so React falls back to the index, as before).
export const keyOf = (v: unknown): string | number | undefined =>
  typeof v === "string" || typeof v === "number" ? v : undefined;

// A loose value that is a string, or undefined (keeps "absent" distinct
// from "empty" where an optional string parameter is expected).
export const optText = (v: unknown): string | undefined =>
  typeof v === "string" ? v : undefined;
