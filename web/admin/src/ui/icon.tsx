import type { LucideIcon, LucideProps } from "lucide-react";
import { cx } from "./utils";

// Named icon sizes and stroke weights: the --ui-icon-* tokens
// (src/theme/sizes.css) size the SVG through its class, so no screen passes
// a pixel size to an icon.
export type IconSize =
  | "2xs"
  | "xs"
  | "sm"
  | "md"
  | "lg"
  | "xl"
  | "2xl"
  | "3xl"
  | "4xl"
  | "5xl"
  | "6xl";
export type IconStroke = "light" | "bold" | "heavy";

// A Lucide icon at a token size: <Icon icon={Plus} size="lg" aria-hidden />.
export function Icon({
  icon: Glyph,
  size,
  stroke,
  className,
  ...props
}: Omit<LucideProps, "size" | "strokeWidth"> & {
  icon: LucideIcon;
  size: IconSize;
  stroke?: IconStroke;
}) {
  return (
    <Glyph
      className={cx(
        "ui-icon-" + size,
        stroke && "ui-icon-stroke-" + stroke,
        className,
      )}
      {...props}
    />
  );
}
