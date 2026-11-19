import type { Meta, StoryObj } from "@storybook/react-vite";
import { Check, Plus, Settings } from "lucide-react";
import { Icon, type IconSize, type IconStroke } from "../icon";

// icon.tsx: a Lucide icon at a named token size (--ui-icon-*); screens never
// pass a pixel size. "lg" (16 px) is the size beside button and body text.
const sizes: IconSize[] = [
  "2xs",
  "xs",
  "sm",
  "md",
  "lg",
  "xl",
  "2xl",
  "3xl",
  "4xl",
  "5xl",
  "6xl",
];
const strokes: IconStroke[] = ["light", "bold", "heavy"];

const meta = { title: "Primitives/Icon" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Sizes: Story = {
  render: () => (
    <ul className="flex flex-wrap items-end gap-4">
      {sizes.map((size) => (
        <li key={size} className="grid justify-items-center gap-1">
          <Icon icon={Plus} size={size} aria-hidden />
          <code>{size}</code>
        </li>
      ))}
    </ul>
  ),
};

export const Strokes: Story = {
  render: () => (
    <ul className="flex flex-wrap items-end gap-4">
      {strokes.map((stroke) => (
        <li key={stroke} className="grid justify-items-center gap-1">
          <Icon icon={Settings} size="xl" stroke={stroke} aria-hidden />
          <code>{stroke}</code>
        </li>
      ))}
      <li className="grid justify-items-center gap-1">
        <Icon icon={Check} size="xl" aria-label="Done" />
        <code>labelled</code>
      </li>
    </ul>
  ),
};
