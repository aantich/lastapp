import { useState } from "react";

const newKey = () => crypto.randomUUID();

// Stable React keys for an editable list whose items have no id of their own
// and must not get one (they are sent to the API as they are). A key is made
// when an item is added and follows its item when an earlier one is removed,
// so inputs keep their state and focus. Call remove(i) together with the
// change that removes item i.
export function useListKeys(length: number) {
  const [keys, setKeys] = useState(() => Array.from({ length }, newKey));
  let current = keys;
  if (keys.length !== length) {
    // Items were added or replaced from outside: extend or trim to match.
    current =
      keys.length < length
        ? [...keys, ...Array.from({ length: length - keys.length }, newKey)]
        : keys.slice(0, length);
    setKeys(current);
  }
  return {
    keys: current,
    remove: (index: number) => setKeys((k) => k.filter((_, j) => j !== index)),
  };
}
