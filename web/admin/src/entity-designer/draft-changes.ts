import { isItem, text, type Item } from "../api-types";

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

// The draft's changes to the active version: null before the first
// publication, 0 when publishing would change nothing. A saved draft is
// compared by definition hash, as the backend decides (it rejects publishing
// an unchanged draft); unsaved edits by the field-by-field diff.
export function publishableChanges(
  draft: Item,
  record: Item,
  dirty: boolean,
): number | null {
  const n = draftChanges(draft, record.active_definition);
  const hash = (v: unknown) => (isItem(v) ? text(v.definition_hash) : "");
  const saved = hash(record.draft_contract),
    live = hash(record.published_contract);
  if (n === null || dirty || !saved || !live) return n;
  return saved === live ? 0 : Math.max(n, 1);
}
