import { useEffect, useState, useRef } from "react";
export function useConsoleURL() {
  const [path, setPath] = useState(location.pathname);
  const current = useRef(location.pathname + location.search);
  current.current = path + location.search;
  useEffect(() => {
    const pop = () => {
      if (
        !window.dispatchEvent(
          new Event("lastapp:before-navigate", { cancelable: true }),
        )
      ) {
        history.pushState(null, "", current.current);
        return;
      }
      setPath(location.pathname);
    };
    window.addEventListener("popstate", pop);
    return () => window.removeEventListener("popstate", pop);
  }, []);
  return [
    path,
    (next: string, replace = false) => {
      if (next !== location.pathname) {
        if (
          !window.dispatchEvent(
            new Event("lastapp:before-navigate", { cancelable: true }),
          )
        )
          return;
        history[replace ? "replaceState" : "pushState"](null, "", next);
        setPath(next);
      }
    },
  ] as const;
}
