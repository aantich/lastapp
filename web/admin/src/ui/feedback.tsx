import type { CSSProperties, ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { cx } from "./utils";
import { Icon, type IconSize } from "./icon";
import type { AvatarSize } from "./avatar";

// An indeterminate wait. The label is announced (role=status); `hideLabel`
// keeps it for screen readers only.
export function Spinner({
  label = "Loading…",
  hideLabel,
  size = "lg",
}: {
  label?: ReactNode;
  hideLabel?: boolean;
  size?: IconSize;
}) {
  return (
    <span className="ui-spinner" role="status">
      <Icon icon={LoaderCircle} className="spin" size={size} aria-hidden />
      <span className={hideLabel ? "ui-sr-only" : undefined}>{label}</span>
    </span>
  );
}

// A placeholder block while content loads; hidden from assistive tech (pair
// it with a Spinner label or aria-busy on the region). It is a text line of a
// share of the width, a short "value" (a count), a taller "block", or a
// circle the size of an avatar.
export function Skeleton({
  width = "100%",
  height = "line",
  circle,
  className,
}: {
  width?: `${number}%` | "value";
  height?: "line" | "block";
  circle?: AvatarSize;
  className?: string;
}) {
  const style: CSSProperties = circle
    ? {
        width: `var(--ui-avatar-${circle})`,
        height: `var(--ui-avatar-${circle})`,
      }
    : {
        width: width === "value" ? "var(--ui-skeleton-value)" : width,
        height: `var(--ui-skeleton-${height})`,
      };
  return (
    <span
      className={cx("ui-skeleton", circle && "ui-circle", className)}
      style={style}
      aria-hidden
    />
  );
}

// A keyboard key or shortcut part: <Kbd>⌘</Kbd><Kbd>K</Kbd>.
export function Kbd({ children }: { children: ReactNode }) {
  return <kbd className="ui-kbd">{children}</kbd>;
}
