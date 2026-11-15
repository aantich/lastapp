import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

// The one place that turns a caught value into the text a screen shows.
export function errorMessage(e: unknown): string {
  if (e && typeof e === "object" && "message" in e) return String(e.message);
  return String(e);
}

// Runs one screen mutation at a time. A second call while one is in flight is
// ignored (double-submit guard). The previous error is cleared when an action
// starts. Results are applied only if the component is still mounted and the
// `context` (usually the route) is the one the action started in, so a slow
// response cannot overwrite a screen the user has already left.
export function useAction(
  context: unknown,
  {
    onSuccess,
    onError,
  }: { onSuccess: () => void; onError: (message: string) => void },
) {
  const [busy, setBusy] = useState(false);
  const running = useRef(false);
  const mounted = useRef(true);
  const latest = useRef({ context, onSuccess, onError });
  useLayoutEffect(() => {
    latest.current = { context, onSuccess, onError };
  });
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const run = useCallback(async (fn: () => Promise<unknown>) => {
    if (running.current) return;
    running.current = true;
    setBusy(true);
    const started = latest.current.context;
    const current = () =>
      mounted.current && Object.is(latest.current.context, started);
    latest.current.onError("");
    try {
      await fn();
      if (current()) latest.current.onSuccess();
    } catch (e) {
      if (current()) latest.current.onError(errorMessage(e));
    } finally {
      running.current = false;
      if (mounted.current) setBusy(false);
    }
  }, []);
  return { busy, run };
}
