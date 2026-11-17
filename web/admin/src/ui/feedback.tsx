import type { CSSProperties, ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { cx } from "./utils";

// An indeterminate wait. The label is announced (role=status); `hideLabel`
// keeps it for screen readers only.
export function Spinner({
  label = "Loading…",
  hideLabel,
  size = 16,
}: {
  label?: ReactNode;
  hideLabel?: boolean;
  size?: number;
}) {
  return (
    <span className="ui-spinner" role="status">
      <LoaderCircle className="spin" size={size} aria-hidden />
      <span className={hideLabel ? "ui-sr-only" : undefined}>{label}</span>
    </span>
  );
}

// A placeholder block while content loads; hidden from assistive tech (pair
// it with a Spinner label or aria-busy on the region).
export function Skeleton({
  width = "100%",
  height = 12,
  circle,
  className,
}: {
  width?: CSSProperties["width"];
  height?: CSSProperties["height"];
  circle?: boolean;
  className?: string;
}) {
  return (
    <span
      className={cx("ui-skeleton", circle && "ui-circle", className)}
      style={{ width, height }}
      aria-hidden
    />
  );
}

// A keyboard key or shortcut part: <Kbd>⌘</Kbd><Kbd>K</Kbd>.
export function Kbd({ children }: { children: ReactNode }) {
  return <kbd className="ui-kbd">{children}</kbd>;
}
