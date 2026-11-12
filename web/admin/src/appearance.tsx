import { useEffect, useRef, useState } from "react";
import { Check, Monitor, Moon, Palette, RotateCcw, Sun, X } from "lucide-react";
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
type Mode = "light" | "dark" | "system";
type Appearance = { theme: Theme; mode: Mode };
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

// Shared by the appearance drawer, navigation search and existing edit forms.
export function useDialogFocus(close: () => void, active = true) {
  const ref = useRef<HTMLElement>(null);
  const onClose = useRef(close);
  onClose.current = close;
  useEffect(() => {
    if (!active) return;
    const previous = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const focusable = () =>
      Array.from(
        ref.current?.querySelectorAll<HTMLElement>(
          'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), [tabindex="0"]',
        ) || [],
      ).filter((el) => el.getClientRects().length);
    (
      ref.current?.querySelector<HTMLElement>("[data-autofocus]") ||
      focusable()[0] ||
      ref.current
    )?.focus();
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose.current();
      }
      if (event.key !== "Tab") return;
      const elements = focusable();
      const first = elements[0],
        last = elements.at(-1);
      if (!first) {
        event.preventDefault();
        return;
      }
      if (
        event.shiftKey &&
        (document.activeElement === first ||
          !ref.current?.contains(document.activeElement))
      ) {
        event.preventDefault();
        last?.focus();
      } else if (
        !event.shiftKey &&
        (document.activeElement === last ||
          !ref.current?.contains(document.activeElement))
      ) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("keydown", handleKey);
      document.body.style.overflow = overflow;
      if (previous?.isConnected) previous.focus();
    };
  }, [active]);
  return ref;
}

export function AppearancePanel({
  value,
  onChange,
  reset,
  close,
}: {
  value: Appearance;
  onChange: (value: Appearance) => void;
  reset: () => void;
  close: () => void;
}) {
  const ref = useDialogFocus(close);
  return (
    <div
      className="overlay drawer-overlay"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) close();
      }}
    >
      <section
        ref={ref}
        className="appearance-drawer bg-surface text-ink"
        role="dialog"
        aria-modal="true"
        aria-labelledby="appearance-title"
        tabIndex={-1}
      >
        <header className="drawer-header">
          <div className="flex items-center gap-3">
            <Palette size={20} className="text-primary" />
            <h2 id="appearance-title">Appearance</h2>
          </div>
          <button
            className="icon"
            aria-label="Close appearance"
            onClick={close}
          >
            <X size={20} />
          </button>
        </header>
        <div className="drawer-body">
          <p className="drawer-intro text-muted text-sm leading-6">
            Make the workspace your own. Changes apply instantly and are saved
            on this browser.
          </p>
          <fieldset>
            <legend>Color mode</legend>
            <div className="mode-options">
              {[
                { id: "light", name: "Light", Icon: Sun },
                { id: "dark", name: "Dark", Icon: Moon },
                { id: "system", name: "System", Icon: Monitor },
              ].map(({ id, name, Icon }) => (
                <label key={id} className={value.mode === id ? "chosen" : ""}>
                  <input
                    type="radio"
                    name="color-mode"
                    value={id}
                    checked={value.mode === id}
                    onChange={() => onChange({ ...value, mode: id as Mode })}
                  />
                  <Icon size={21} />
                  <span>{name}</span>
                </label>
              ))}
            </div>
          </fieldset>
          <fieldset>
            <legend>Theme preset</legend>
            <div className="theme-options">
              {themes.map((theme) => (
                <label
                  key={theme.id}
                  className={value.theme === theme.id ? "chosen" : ""}
                >
                  <input
                    type="radio"
                    name="theme-preset"
                    value={theme.id}
                    checked={value.theme === theme.id}
                    onChange={() => onChange({ ...value, theme: theme.id })}
                  />
                  <span
                    className={"theme-preview preview-" + theme.id}
                    aria-hidden="true"
                  >
                    <i />
                    <span>
                      <b />
                      <b />
                      <b />
                    </span>
                  </span>
                  <span className="theme-description">
                    <strong>
                      {theme.name}
                      {theme.id === "neo" && <em>Default</em>}
                    </strong>
                    <small>{theme.description}</small>
                  </span>
                  {value.theme === theme.id && (
                    <Check size={17} className="text-primary" />
                  )}
                </label>
              ))}
            </div>
          </fieldset>
          <div className="rounded-panel border border-line bg-subtle p-4 text-sm text-muted leading-6">
            Your theme follows you across all console screens, forms and
            resource tables.
          </div>
        </div>
        <footer className="drawer-footer">
          <button className="secondary" onClick={reset}>
            <RotateCcw size={15} />
            Reset to default
          </button>
          <button className="primary" onClick={close}>
            Done
            <Check size={15} />
          </button>
        </footer>
      </section>
    </div>
  );
}
