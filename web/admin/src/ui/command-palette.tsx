import { useMemo, useState, type ReactNode } from "react";
import { Search } from "lucide-react";
import { useListbox } from "./listbox";
import { useDialogFocus } from "./use-dialog-focus";

export type Command = {
  id: string;
  label: string;
  // Its kind, shown at the row's end: Navigate, Entities, Apps, Action…
  group: string;
  icon?: ReactNode;
  // Extra words that match the filter (IDs, synonyms).
  keywords?: string;
  hint?: ReactNode;
  disabled?: boolean;
  run: () => void;
};

// Search words that find real commands, quoted in the placeholder and the
// empty state ("try “deal”, “deploy”, “agent”").
const quoted = (words: string[]) => words.map((w) => `“${w}”`);

// The ⌘K command palette: a modal dialog with a filter combobox over a
// listbox of commands, each with its glyph, label and kind. ↑/↓ move
// (skipping disabled commands), Enter runs, Escape, the Esc button or a press
// outside closes and focus returns to the opener. Keyboard state is the
// shared useListbox.
export function CommandPalette({
  commands,
  onClose,
  label = "Command palette",
  examples = [],
}: {
  commands: Command[];
  onClose: () => void;
  label?: string;
  examples?: string[];
}) {
  const [query, setQuery] = useState("");
  const shown = useMemo(() => {
    const words = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
    return commands
      .filter((c) => {
        const haystack =
          `${c.label} ${c.group} ${c.keywords || ""}`.toLowerCase();
        return words.every((w) => haystack.includes(w));
      })
      .map((c) => ({ ...c, value: c.id }));
  }, [commands, query]);
  const ref = useDialogFocus(onClose);
  // Always open while mounted; closing the list (Escape, a press outside the
  // panel) closes the palette.
  const list = useListbox(shown, run, ref, [
    true,
    (next) => {
      if (next === false) onClose();
    },
  ]);
  function run(command: Command) {
    onClose();
    command.run();
  }
  return (
    <div className="overlay ui-palette-overlay">
      <section
        ref={ref}
        className="ui-palette"
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
      >
        <div className="ui-palette-input">
          <Search size={18} aria-hidden />
          <input
            data-autofocus
            role="combobox"
            aria-label="Search commands"
            aria-expanded
            aria-controls={list.id}
            aria-autocomplete="list"
            aria-activedescendant={list.activeId}
            placeholder={
              "Type a command or search…" +
              (examples.length ? " try " + quoted(examples).join(", ") : "")
            }
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              list.setActive(list.step(-1, 1));
            }}
            onKeyDown={(e) => list.onKeyDown(e)}
          />
          <button type="button" className="ui-palette-close" onClick={onClose}>
            Esc
          </button>
        </div>
        <div role="listbox" id={list.id} aria-label="Commands">
          {shown.map((c, i) => (
            <div
              key={c.id}
              id={`${list.id}-${i}`}
              role="option"
              aria-selected={list.active === i}
              aria-disabled={c.disabled || undefined}
              data-active={list.active === i || undefined}
              className="ui-option"
              onMouseDown={(e) => e.preventDefault()}
              onMouseMove={() => list.setActive(i)}
              onClick={() => {
                if (!c.disabled) run(c);
              }}
            >
              <i aria-hidden>{c.icon}</i>
              <span>
                {c.label}
                {c.hint && <small>{c.hint}</small>}
              </span>
              <small className="ui-palette-kind">{c.group}</small>
            </div>
          ))}
          {!shown.length && (
            <p className="ui-palette-empty" role="status">
              Nothing matches.
              {examples.length > 1 &&
                ` Try ${quoted(examples.slice(-2)).join(" or ")}.`}
            </p>
          )}
        </div>
      </section>
    </div>
  );
}
