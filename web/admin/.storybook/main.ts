import type { StorybookConfig } from "@storybook/react-vite";

// Local only: `npm run storybook` / `npm run build-storybook`. Stories use the
// console's own Vite config (Tailwind, React) and real stylesheets.
const config: StorybookConfig = {
  framework: "@storybook/react-vite",
  stories: ["../src/**/*.mdx", "../src/**/*.stories.@(ts|tsx)"],
  addons: ["@storybook/addon-docs", "@storybook/addon-a11y"],
  core: { disableTelemetry: true },
  // The console is served under /console/; Storybook is served from its root.
  viteFinal: (vite) => ({ ...vite, base: "./" }),
};
export default config;
