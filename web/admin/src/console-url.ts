import { useEffect, useState, useRef } from "react";

// Dispatched before the console changes route. A listener that must ask the
// user first calls preventDefault() and later calls detail.proceed() to go on.
export type BeforeNavigateEvent = CustomEvent<{ proceed: () => void }>;
export const beforeNavigateEvent = "lastapp:before-navigate";
const navigationAllowed = (proceed: () => void) =>
  window.dispatchEvent(
    new CustomEvent(beforeNavigateEvent, {
      cancelable: true,
      detail: { proceed },
    }),
  );

export function useConsoleURL() {
  const [path, setPath] = useState(location.pathname);
  const current = useRef(location.pathname + location.search);
  current.current = path + location.search;
  useEffect(() => {
    const pop = () => {
      const target = location.pathname + location.search;
      const proceed = () => {
        if (location.pathname + location.search !== target)
          history.pushState(null, "", target);
        setPath(location.pathname);
      };
      if (navigationAllowed(proceed)) {
        setPath(location.pathname);
        return;
      }
      history.pushState(null, "", current.current);
    };
    window.addEventListener("popstate", pop);
    return () => window.removeEventListener("popstate", pop);
  }, []);
  return [
    path,
    (next: string, replace = false) => {
      if (next === location.pathname) return;
      const proceed = () => {
        history[replace ? "replaceState" : "pushState"](null, "", next);
        setPath(next);
      };
      if (navigationAllowed(proceed)) proceed();
    },
  ] as const;
}
