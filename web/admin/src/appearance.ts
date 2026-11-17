import { useEffect, useState } from "react";
import { savePreference } from "./preferences";

// Signal is the only theme; the browser preference chooses the color mode.
export const modes = ["light", "dark", "system"] as const;
export type Mode = (typeof modes)[number];
export type Appearance = { mode: Mode };
const storageKey = "lastapp-appearance";
const defaults: Appearance = { mode: "light" };
// Browser chrome color: the Signal canvas of each resolved mode.
const chrome = { light: "#f6f5f2", dark: "#141518" };

// Older saved values also carry a `theme` preset; it is ignored.
function readAppearance(): Appearance {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || "null");
    return { mode: modes.includes(saved?.mode) ? saved.mode : defaults.mode };
  } catch {
    return defaults;
  }
}

export function applyMode(mode: Mode) {
  const dark =
    mode === "dark" ||
    (mode === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.mode = dark ? "dark" : "light";
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", dark ? chrome.dark : chrome.light);
}

// Apply before React mounts; index.html also sets the attribute before CSS loads.
applyMode(readAppearance().mode);

export function useAppearance() {
  const [appearance, setAppearance] = useState<Appearance>(readAppearance);
  useEffect(() => {
    applyMode(appearance.mode);
    savePreference(storageKey, JSON.stringify(appearance));
    const system = matchMedia("(prefers-color-scheme: dark)");
    const updateSystem = () => applyMode(appearance.mode);
    system.addEventListener("change", updateSystem);
    return () => system.removeEventListener("change", updateSystem);
  }, [appearance]);
  useEffect(() => {
    const sync = (event: StorageEvent) => {
      if (event.key === storageKey || event.key === null)
        setAppearance(readAppearance());
    };
    window.addEventListener("storage", sync);
    return () => window.removeEventListener("storage", sync);
  }, []);
  return {
    appearance,
    setAppearance,
    reset: () => setAppearance({ ...defaults }),
  };
}
