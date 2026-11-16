import { useEffect, useState } from "react";
import { savePreference } from "./preferences";

export const themes = [
  { id: "neo", name: "Neo", description: "Quiet blue · fine dashed borders" },
  {
    id: "studio",
    name: "Studio",
    description: "Soft violet · rounded surfaces",
  },
  {
    id: "evergreen",
    name: "Evergreen",
    description: "Natural green · crisp and calm",
  },
] as const;
type Theme = (typeof themes)[number]["id"];
export type Mode = "light" | "dark" | "system";
export type Appearance = { theme: Theme; mode: Mode };
const storageKey = "lastapp-appearance";
const defaults: Appearance = { theme: "neo", mode: "light" };

function readAppearance(): Appearance {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || "null");
    return {
      theme: themes.some((t) => t.id === saved?.theme)
        ? saved.theme
        : defaults.theme,
      mode: ["light", "dark", "system"].includes(saved?.mode)
        ? saved.mode
        : defaults.mode,
    };
  } catch {
    return defaults;
  }
}

function applyAppearance(value: Appearance) {
  const dark =
    value.mode === "dark" ||
    (value.mode === "system" &&
      matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = value.theme;
  document.documentElement.dataset.mode = dark ? "dark" : "light";
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", dark ? "#17181e" : "#ffffff");
}

// Apply before React mounts; index.html also sets the attributes before CSS loads.
applyAppearance(readAppearance());

export function useAppearance() {
  const [appearance, setAppearance] = useState<Appearance>(readAppearance);
  const [systemDark, setSystemDark] = useState(
    () => matchMedia("(prefers-color-scheme: dark)").matches,
  );
  useEffect(() => {
    applyAppearance(appearance);
    savePreference(storageKey, JSON.stringify(appearance));
    const system = matchMedia("(prefers-color-scheme: dark)");
    const updateSystem = () => {
      setSystemDark(system.matches);
      applyAppearance(appearance);
    };
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
    isDark:
      appearance.mode === "dark" ||
      (appearance.mode === "system" && systemDark),
    reset: () => setAppearance({ ...defaults }),
  };
}
