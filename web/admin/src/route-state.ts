import { useCallback, useState } from "react";

// Screen state that belongs to one route (an open modal, an error banner).
// When `route` changes, the value reads as `initial` again without an effect,
// and a late set() from the previous route cannot leak onto the new one.
export function useRouteState<T>(route: unknown, initial: T) {
  const [state, setState] = useState<{ route: unknown; value: T }>({
    route,
    value: initial,
  });
  const value = Object.is(state.route, route) ? state.value : initial;
  const set = useCallback((value: T) => setState({ route, value }), [route]);
  return [value, set] as const;
}

// A screen's error banner: the latest action error, or else the first read
// error not yet dismissed. Dismissing hides that read error until the read
// fails again.
export function useScreenError(
  route: unknown,
  readErrors: (Error | null | undefined)[],
) {
  const [error, setError] = useRouteState(route, ""),
    [dismissed, setDismissed] = useState<Error | null>(null);
  const read = readErrors.find((e) => e && e !== dismissed) || null;
  return {
    error: error || read?.message || "",
    setError,
    dismiss: () => {
      setError("");
      setDismissed(read);
    },
  };
}
