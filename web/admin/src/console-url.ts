import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useState,
  useRef,
} from "react";

// The console's route table: every console URL is built and parsed here.
//   /console/                                   sign-in landing
//   /console/launch, /console/agent-consent     standalone flows
//   /console/account                            Tenant member account
//   /console/tenants/{tenant}/{section}/{id}/{tab}
//   /console/platform/{page}/{id}/{tab}
const root = "/console/";
const join = (base: string, path: string[]) =>
  path.length ? base + "/" + path.join("/") : base;
export const consoleURL = {
  root,
  launch: root + "launch",
  agentConsent: root + "agent-consent",
  account: root + "account",
  tenant: (tenant: string, ...path: string[]) =>
    join(root + "tenants/" + tenant, path),
  platform: (...path: string[]) => root + "platform/" + path.join("/"),
};
// Only a single-slash console path is a safe place to return to.
export const isConsolePath = (path: string | null | undefined) =>
  !!path?.startsWith(root) && !path.startsWith(root + "/");

// Path segments after "/console", without the query string. Empty segments
// are dropped.
const segments = (path: string) =>
  path.split("?")[0].split("/").filter(Boolean).slice(1);

export function parseTenantURL(path: string) {
  const [area = "", tenant = "", section = "", id = "", tab = ""] =
    segments(path);
  return {
    // "tenants", "account", or anything else (redirected by the workspace).
    area,
    tenant: area === "tenants" ? tenant : "",
    page: area === "account" ? "account" : section || "home",
    space: section === "spaces" ? id : "",
    app: section === "apps" ? id : "",
    tab:
      section === "spaces" && id
        ? tab || "apps"
        : section === "apps" && id
          ? tab || "releases"
          : id || "general",
    team: section === "people" ? tab : "",
  };
}

export function parsePlatformURL(path: string) {
  const [, page = "tenants", id = "", tab = ""] = segments(path);
  return {
    page,
    // The tenant, settings tab or activity tenant, depending on page.
    id,
    tenant: page === "tenants" ? id : "",
    tab: tab || "overview",
    settingsTab: id || "servers",
  };
}

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

// The current console location: the path and, when a link carries one (an
// entity opened by ?entity=), its query string.
const here = () => location.pathname + location.search;

export function useConsoleURL() {
  const [path, setPath] = useState(here);
  // The URL to restore when a listener cancels a back/forward navigation.
  const current = useRef(here());
  useLayoutEffect(() => {
    current.current = here();
  });
  useEffect(() => {
    const pop = () => {
      const target = here();
      const proceed = () => {
        if (here() !== target) history.pushState(null, "", target);
        setPath(target);
      };
      if (navigationAllowed(proceed)) {
        setPath(target);
        return;
      }
      history.pushState(null, "", current.current);
    };
    window.addEventListener("popstate", pop);
    return () => window.removeEventListener("popstate", pop);
  }, []);
  // Stable, so screens can redirect from an effect that lists it.
  const navigate = useCallback((next: string, replace = false) => {
    if (next === here()) return;
    const proceed = () => {
      history[replace ? "replaceState" : "pushState"](null, "", next);
      setPath(next);
    };
    if (navigationAllowed(proceed)) proceed();
  }, []);
  return [path, navigate] as const;
}
