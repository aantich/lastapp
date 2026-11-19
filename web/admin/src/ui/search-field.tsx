import { Search } from "lucide-react";
import { Tooltip } from "./tooltip";
import { cx } from "./utils";

const shortcut = /Mac|iPhone|iPad/.test(navigator.userAgent) ? "⌘K" : "Ctrl K";

// A search field that opens the command palette and shows the platform's
// shortcut (⌘K or Ctrl K): wide in the top bar, on ink in the sidebar, an
// icon on the collapsed rail. The label stays in the accessible name when a
// narrow screen hides it.
export function SearchField({
  label,
  onOpen,
  variant = "topbar",
  rail,
}: {
  label: string;
  onOpen: () => void;
  variant?: "topbar" | "sidebar";
  rail?: boolean;
}) {
  const button = (
    <button
      type="button"
      className={cx("ui-search-field", variant === "sidebar" && "ui-on-ink")}
      aria-keyshortcuts="Meta+K Control+K"
      aria-label={rail ? label.replace("…", "") + ` (${shortcut})` : undefined}
      onClick={onOpen}
    >
      <Search
        size={variant === "sidebar" ? (rail ? 18 : 15) : 16}
        aria-hidden
      />
      <span className="ui-search-label">{label}</span>
      <kbd className="ui-search-key" aria-hidden>
        {shortcut}
      </kbd>
    </button>
  );
  return rail ? (
    <Tooltip content={label.replace("…", "")} side="right" labelOnly>
      {button}
    </Tooltip>
  ) : (
    button
  );
}
