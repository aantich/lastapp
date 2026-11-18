import type { ReactNode } from "react";
import { cx } from "./utils";
import { statusTone, type Tone } from "./status-tone";

// A lifecycle state as a pill with a tone-specific mark (filled dot, half
// dot, diamond, ring, dash), so tones are told apart without color. The
// label defaults to the state itself, shown capitalized.
export function StatusBadge({
  state,
  label,
  tone,
  className,
}: {
  state: unknown;
  label?: ReactNode;
  // Overrides the map, for a state the map cannot know (rare).
  tone?: Tone;
  className?: string;
}) {
  const value = state === null || state === undefined ? "" : String(state);
  const t = tone || statusTone(value);
  return (
    <span
      className={cx("badge", "tone-" + t, className)}
      data-state={value || undefined}
    >
      <i aria-hidden />
      {label ?? (
        // The state word itself; its capital is presentation (CSS).
        <span className="ui-badge-state">{value.replace(/_/g, " ")}</span>
      )}
    </span>
  );
}

// A compact label: a tag ("read", "acl.manage"), a mono identifier or a
// count. Not interactive.
export function Badge({
  children,
  tone = "neutral",
  mono,
  count,
  pill,
  className,
  title,
}: {
  children: ReactNode;
  tone?: Tone | "inverse";
  mono?: boolean;
  count?: boolean;
  // The rounder, larger chip of cards (a preset, a permission, a scope).
  pill?: boolean;
  className?: string;
  title?: string;
}) {
  return (
    <span
      className={cx(
        "ui-tag",
        tone !== "neutral" && "tone-" + tone,
        mono && "ui-mono",
        count && "ui-count",
        pill && "ui-pill",
        className,
      )}
      title={title}
    >
      {children}
    </span>
  );
}
