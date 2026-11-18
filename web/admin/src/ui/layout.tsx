import type { ReactNode } from "react";
import { cx } from "./utils";

type Gap = "sm" | "md" | "lg";

// A screen's vertical rhythm: cards and sections one above the other.
export function Stack({
  gap = "lg",
  children,
  className,
}: {
  gap?: Gap;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cx("ui-stack", "ui-gap-" + gap, className)}>{children}</div>
  );
}

// A row of controls or badges that wraps on narrow screens.
export function Inline({
  gap = "sm",
  children,
  className,
}: {
  gap?: Gap;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cx("ui-inline", "ui-gap-" + gap, className)}>
      {children}
    </div>
  );
}

// A record list beside the open record (release history | release): the
// aside takes a quarter of the width and stacks above the main part under
// 800 px.
export function Split({
  aside,
  children,
}: {
  aside: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="ui-split">
      <div className="ui-split-aside">{aside}</div>
      <div className="ui-split-main">{children}</div>
    </div>
  );
}
