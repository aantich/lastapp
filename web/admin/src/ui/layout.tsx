import type { CSSProperties, ReactNode } from "react";
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

// Cards side by side: equal columns of at least `min` px that wrap as the
// screen narrows (a grid of cards), or fixed `weights` such as "2fr 1fr"
// (a main column and a side column) that stack under 800 px.
export function Columns({
  min = 280,
  weights,
  gap = "lg",
  children,
  className,
}: {
  min?: number;
  weights?: string;
  gap?: Gap;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cx(
        "ui-columns",
        weights && "ui-columns-weighted",
        "ui-gap-" + gap,
        className,
      )}
      style={
        {
          "--ui-columns-min": min + "px",
          ...(weights && { "--ui-columns-weights": weights }),
        } as CSSProperties
      }
    >
      {children}
    </div>
  );
}
