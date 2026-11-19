// Named minimum widths (the --ui-measure-* tokens in src/theme/sizes.css):
// how narrow a card column, a weighted track or a pipeline step gets before
// its row wraps or stacks.
export type Measure =
  "3xs" | "2xs" | "xs" | "sm" | "md" | "lg" | "xl" | "2xl" | "3xl";

export const measure = (name: Measure) => `var(--ui-measure-${name})`;
