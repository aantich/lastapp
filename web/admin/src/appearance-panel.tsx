import { Check, Monitor, Moon, Palette, RotateCcw, Sun, X } from "lucide-react";
import { useDialogFocus } from "./dialog";
import type { Appearance, Mode } from "./appearance";

const modeOptions: { id: Mode; name: string; Icon: typeof Sun }[] = [
  { id: "light", name: "Light", Icon: Sun },
  { id: "dark", name: "Dark", Icon: Moon },
  { id: "system", name: "System", Icon: Monitor },
];

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
              {modeOptions.map(({ id, name, Icon }) => (
                <label key={id} className={value.mode === id ? "chosen" : ""}>
                  <input
                    type="radio"
                    name="color-mode"
                    value={id}
                    checked={value.mode === id}
                    onChange={() => onChange({ ...value, mode: id })}
                  />
                  <Icon size={21} />
                  <span>{name}</span>
                </label>
              ))}
            </div>
          </fieldset>
          <div className="rounded-panel border border-line bg-subtle p-4 text-sm text-muted leading-6">
            The Signal theme follows you across all console screens, forms and
            resource tables. System mode follows your device setting.
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
