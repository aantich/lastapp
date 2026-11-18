import { Children, type CSSProperties, type ReactNode } from "react";
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
// (a main column and a side column) that stack under 800 px. `tracks`
// gives each child its own share and minimum width instead, e.g.
// [{ grow: 8, min: 480 }, { grow: 5, min: 320 }]: the columns stack as soon
// as the space cannot hold both minimums, whatever the viewport.
export function Columns({
  min = 280,
  weights,
  tracks,
  gap = "lg",
  children,
  className,
}: {
  min?: number;
  weights?: string;
  tracks?: { grow: number; min: number }[];
  gap?: Gap;
  children: ReactNode;
  className?: string;
}) {
  if (tracks)
    return (
      <div className={cx("ui-columns-tracks", "ui-gap-" + gap, className)}>
        {Children.toArray(children).map((child, i) => (
          <div
            key={i}
            style={{
              flexGrow: tracks[i]?.grow ?? 1,
              flexBasis: (tracks[i]?.min ?? min) + "px",
            }}
          >
            {child}
          </div>
        ))}
      </div>
    );
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
