// Same rule set as forge/internal/ui: JS + typescript-eslint recommended,
// React hooks (incl. exhaustive-deps) and react-refresh for Vite, plus the
// console's UI code rules (DEVELOPMENT.md, "UI code rules"). Stylesheets are
// checked by scripts/check-css.mjs, which `npm run lint` also runs.
import js from "@eslint/js";
import { defineConfig, globalIgnores } from "eslint/config";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";
import tseslint from "typescript-eslint";

// Visual values come from the Signal tokens (src/theme), never literals.
// esquery regexes cannot contain "/", so these patterns avoid it.
const literalRules = [
  ...["Literal", "TemplateElement > .value"].flatMap((node) => {
    const value = node === "Literal" ? "value" : "raw";
    return [
      {
        selector: `${node}[${value}=/#[0-9a-fA-F]{3,8}\\b|\\b(rgba?|hsla?|hwb|lab|lch|oklab|oklch)\\(/]`,
        message: "Color literal: use a --ui-* color token.",
      },
      {
        selector: `${node}[${value}=/(^|[^\\w.-])-?[0-9.]+(px|rem|em|vh|vw|ms)\\b/]`,
        message:
          "Literal CSS length or duration: use a --ui-* token or a named kit size.",
      },
      {
        selector: `${node}[${value}=/(^|[\\s:!])-?[a-z][\\w-]*-\\[[^\\]]+\\]|(^|\\s)\\[[a-z-]+:[^\\]]+\\]/]`,
        message:
          "Tailwind arbitrary value: use a theme token utility or a kit component.",
      },
    ];
  }),
  {
    // Icon, avatar and spinner sizes are named kit sizes (Icon, Avatar, ...);
    // shapes inside an inline SVG keep their viewBox coordinates.
    selector:
      "JSXAttribute[name.name=/^(size|width|height|strokeWidth)$/] > JSXExpressionContainer > Literal[raw=/^[0-9.]+$/], JSXOpeningElement:not([name.name=/^(rect|circle|ellipse|line|path|polygon|polyline|g|use)$/]) > JSXAttribute[name.name=/^(size|width|height)$/] > Literal[value=/^[0-9.]+$/]",
    message: "Literal size: use a named kit size (for icons, the kit Icon).",
  },
];

// Screens (everything outside src/ui) are built from kit components only.
const kitOnlyRules = [
  {
    selector:
      "JSXOpeningElement[name.name=/^(button|input|textarea|select|table|dialog)$/]",
    message:
      "Raw form/table/dialog element in a screen: use a src/ui component.",
  },
  {
    selector: "JSXAttribute[name.name='style']",
    message: "Inline style in a screen: use src/ui components and tokens.",
  },
];

export default defineConfig([
  globalIgnores([
    "dist",
    "test-results",
    "playwright-report",
    "storybook-static",
  ]),
  {
    files: ["src/**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: { ecmaVersion: 2022, globals: globals.browser },
    // No eslint-disable comments: a rule is followed or changed here.
    linterOptions: { noInlineConfig: true },
    rules: {
      // Stale closures in effects are real bugs here; fail instead of warning.
      "react-hooks/exhaustive-deps": "error",
      "max-lines": [
        "error",
        { max: 200, skipBlankLines: true, skipComments: true },
      ],
      "no-restricted-syntax": ["error", ...literalRules],
    },
  },
  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/ui/**"],
    rules: {
      "no-restricted-syntax": ["error", ...literalRules, ...kitOnlyRules],
    },
  },
  {
    // Stories demonstrate every state of a kit component in one file; the
    // line limit applies to the components themselves.
    files: ["src/**/*.stories.tsx"],
    rules: { "max-lines": "off" },
  },
  {
    // Local Storybook configuration (stories themselves are under src/).
    files: [".storybook/**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
    ],
    languageOptions: { ecmaVersion: 2022, globals: globals.browser },
  },
  {
    files: ["tests/**/*.ts", "*.config.{ts,js}", "scripts/**/*.mjs"],
    extends: [js.configs.recommended, tseslint.configs.recommended],
    languageOptions: { ecmaVersion: 2022, globals: globals.node },
  },
]);
