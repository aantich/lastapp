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

## Theme contract (superseded)

The Neo, Studio and Evergreen presets described by this reference were replaced
by the single Signal theme in November 2026. The current token contract, color
modes and UI library rules are in the
[console UI library section of DEVELOPMENT.md](../../DEVELOPMENT.md#console-ui-library).

Sources: [Tailwind theme variables](https://tailwindcss.com/docs/theme), [Tailwind dark mode](https://tailwindcss.com/docs/dark-mode).
