import { useMemo, useState, type ReactNode } from "react";
import { CornerDownLeft, Search } from "lucide-react";
import { useListbox } from "./listbox";
import { useDialogFocus } from "./use-dialog-focus";
import { Kbd } from "./feedback";

export type Command = {
  id: string;
  label: string;
  // Group heading: Navigate, Entities, Apps, Actions…
  group: string;
  icon?: ReactNode;
  // Extra words that match the filter (IDs, synonyms).
  keywords?: string;
  hint?: ReactNode;
  disabled?: boolean;
  run: () => void;
};

// The ⌘K command palette: a modal dialog with a filter combobox over a
// grouped listbox. ↑/↓ move (skipping disabled commands), Enter runs, Escape
// or a press outside closes and focus returns to the opener. Keyboard state
// is the shared useListbox.
export function CommandPalette({
  commands,
  onClose,
  label = "Command palette",
  placeholder = "Search or run a command…",
  empty = "Nothing matches.",
}: {
  commands: Command[];
  onClose: () => void;
  label?: string;
  placeholder?: string;
  empty?: ReactNode;
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
  const groups: [string, number[]][] = [];
  shown.forEach((c, i) => {
    const group = groups.find(([name]) => name === c.group);
    if (group) group[1].push(i);
    else groups.push([c.group, [i]]);
  });
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
        <label className="ui-palette-input">
          <Search size={18} aria-hidden />
          <input
            data-autofocus
            role="combobox"
            aria-label={label}
            aria-expanded
            aria-controls={list.id}
            aria-autocomplete="list"
            aria-activedescendant={list.activeId}
            placeholder={placeholder}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              list.setActive(list.step(-1, 1));
            }}
            onKeyDown={(e) => list.onKeyDown(e)}
          />
          <Kbd>Esc</Kbd>
        </label>
        <div role="listbox" id={list.id} aria-label="Commands">
          {groups.map(([group, indexes], g) => (
            <div
              key={group}
              role="group"
              aria-labelledby={`${list.id}-g${g}`}
              className="ui-palette-group"
            >
              <div id={`${list.id}-g${g}`}>{group}</div>
              {indexes.map((i) => {
                const c = shown[i];
                return (
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
                    {c.icon && <i aria-hidden>{c.icon}</i>}
                    <span>
                      {c.label}
                      {c.hint && <small>{c.hint}</small>}
                    </span>
                    {list.active === i && (
                      <CornerDownLeft size={14} aria-hidden />
                    )}
                  </div>
                );
              })}
            </div>
          ))}
          {!shown.length && (
            <div className="ui-option-empty" role="status">
              {empty}
            </div>
          )}
        </div>
        <footer aria-hidden>
          <span>
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd> move
          </span>
          <span>
            <Kbd>↵</Kbd> run
          </span>
          <span>
            <Kbd>Esc</Kbd> close
          </span>
        </footer>
      </section>
    </div>
  );
}
