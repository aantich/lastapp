import { useLayoutEffect, type ReactNode } from "react";
import type { Preview } from "@storybook/react-vite";
import "../src/style.css";
import "../src/agent-access.css";
import { applyMode, modes, type Mode } from "../src/appearance";

// The toolbar mode applies the same data-mode attribute as the console.
function ModeFrame({ mode, children }: { mode: Mode; children: ReactNode }) {
  useLayoutEffect(() => applyMode(mode), [mode]);
  return <>{children}</>;
}

const preview: Preview = {
  globalTypes: {
    mode: {
      description: "Signal color mode",
      toolbar: {
        title: "Mode",
        icon: "contrast",
        items: modes.map((value) => ({ value, title: value })),
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { mode: "light" },
  decorators: [
    (Story, { globals }) => (
      <ModeFrame mode={(globals.mode as Mode) || "light"}>
        <Story />
      </ModeFrame>
    ),
  ],
  parameters: {
    layout: "padded",
    // Show axe results in the panel; there is no Storybook test run in CI.
    a11y: { test: "todo" },
    controls: { expanded: true },
  },
};
export default preview;
