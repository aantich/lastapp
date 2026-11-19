import { useEffect, useState } from "react";
import { readPreference, savePreference } from "../preferences";

// How long typing waits before a search runs, so each keystroke is not a
// request (ms).
const searchDelay = 150;

// The search text once typing pauses for searchDelay, and a setter that
// changes it at once (clearing a search without the delay).
export function useDebouncedSearch(query: string) {
  const [search, setSearch] = useState(query);
  useEffect(() => {
    const timer = setTimeout(() => setSearch(query), searchDelay);
    return () => clearTimeout(timer);
  }, [query]);
  return [search, setSearch] as const;
}

// Opens the palette on ⌘K (macOS) or Ctrl+K anywhere in the console.
export function useCommandPaletteShortcut(open: () => void) {
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        open();
      }
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [open]);
}

const collapseKey = "lastapp-sidebar";

// The desktop sidebar's collapsed (68px rail) state, kept in the browser so
// it survives reloads. The shell puts `sidebar-collapsed` on `.shell`.
export function useSidebarCollapse() {
  const [collapsed, setState] = useState(
    () => readPreference(collapseKey) === "collapsed",
  );
  const setCollapsed = (next: boolean) => {
    setState(next);
    savePreference(collapseKey, next ? "collapsed" : "expanded");
  };
  return [collapsed, setCollapsed] as const;
}
