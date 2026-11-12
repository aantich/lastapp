# Lastapp admin visual reference

Reference inspected **12 November 2026**: [Paces Bootstrap — Neo](https://themes.coderthemes.com/paces/bootstrap/index.html?neo), by Coderthemes.

The requested direction is Paces Neo's layout and visual language, implemented in Lastapp's React/Tailwind frontend. Lastapp retains its real control-plane data and provisioning/access workflows.

## Observed design

- Inter, 13px base; compact 14–16px panel headings, 28–32px metric values.
- 244px pale gray sidebar, grouped uppercase navigation labels, slate active rows.
- 66px white topbar, pill navigation search, appearance controls and identity.
- White workspace, restrained 4px corners, thin `#e2e7ec` dashed card borders, no card shadows.
- Neo palette: blue `#4f7fb3`, mint `#6fbfa5`, amber `#e6c06f`, rose `#e38c8c`; semantic badge text uses darker shades for legibility.
- Modular metrics, circular status visualization, inventory breakdown and compact tables.

Source observations came from the rendered demo, `assets/css/app.min.css` and `demo.js`. Raw HTML, CSS and configuration snapshots are retained locally in ignored `.local/design-reference/paces-neo/`. They are reference material only; the shipped application does not load Paces scripts, Bootstrap, logos, demo data or commercial template assets.

Typography is locally bundled from the [official Inter repository](https://github.com/rsms/inter), under its included SIL Open Font License. The font requires no third-party runtime requests.

## Theme contract

`web/admin/src/themes.css` defines semantic `--ui-*` tokens, with Neo, Studio and Evergreen presets and light/dark overrides. Tailwind 4's `@theme inline` maps these to utilities such as `bg-surface`, `text-ink`, `text-primary`, `border-line`, `rounded-panel` and `shadow-panel`. Existing component classes consume the same variables. Prefer semantic tokens over fixed palette colors when adding UI.

`data-theme` chooses the preset; `data-mode` is the resolved light/dark mode. The custom `dark:` variant uses the latter. The default is Neo/light. Appearance offers Light, Dark and System and saves a validated `{ theme, mode }` preference in browser-local `lastapp-appearance`. System changes and preference changes from other tabs are observed. Invalid values fall back to defaults; disabled storage still allows switching during the session. `public/appearance-init.js` applies validated attributes before CSS loads, through a same-origin script allowed by the existing CSP.

To add a preset, add its ID/name/description in `appearance.tsx`, include its ID in the early bootstrap allowlist in `public/appearance-init.js`, define its token overrides in `themes.css`, and add a small `.preview-*` swatch in `style.css`. Define a dark override when its accent needs different contrast. Keep status colors independent of brand colors.

Sources: [Tailwind theme variables](https://tailwindcss.com/docs/theme), [Tailwind dark mode](https://tailwindcss.com/docs/dark-mode).
