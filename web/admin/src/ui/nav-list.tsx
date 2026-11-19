import type { ReactNode } from "react";

export type NavListItem = {
  id: string;
  label: ReactNode;
  // A dot after the label (e.g. unpublished changes), spoken as `dotLabel`.
  dot?: boolean;
  dotLabel?: string;
};

// A card listing records by name under an uppercase label; the open one is
// aria-current. Pressing a name picks it. The footer holds "+ New …" and
// notes.
export function NavList({
  label,
  items,
  value,
  onPick,
  disabled,
  footer,
}: {
  label: string;
  items: NavListItem[];
  value?: string;
  onPick: (id: string) => void;
  disabled?: boolean;
  footer?: ReactNode;
}) {
  return (
    <nav aria-label={label} className="ui-nav-list panel">
      <span className="ui-nav-list-label">{label}</span>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          aria-current={item.id === value ? "true" : undefined}
          disabled={disabled}
          onClick={() => onPick(item.id)}
        >
          {item.label}
          {item.dot && (
            <span className="ui-nav-list-dot">
              <span className="ui-sr-only">{item.dotLabel}</span>
            </span>
          )}
        </button>
      ))}
      {footer}
    </nav>
  );
}

// A small muted note in a NavList footer ("Templates: …").
export function NavListNote({ children }: { children: ReactNode }) {
  return <span className="ui-nav-list-note">{children}</span>;
}
