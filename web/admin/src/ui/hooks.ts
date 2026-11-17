import { useEffect, useState } from "react";
import { readPreference, savePreference } from "../preferences";

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
