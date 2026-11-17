import type { Meta, StoryObj } from "@storybook/react-vite";
import { contrast, dark, isColor, light, lightness } from "./tokens";

const groups: [string, RegExp][] = [
  [
    "Surfaces and text",
    /^--ui-(canvas|surface|subtle|hover|border|overlay|text|muted|faint)$/,
  ],
  ["Sidebar (ink)", /^--ui-sidebar/],
  ["Signal blue and focus", /^--ui-(primary|on-primary|focus)/],
  ["Status", /^--ui-(success|warning|danger|info|neutral)/],
  [
    "Controls and inverse",
    /^--ui-(control|on-control|skeleton|inverse|on-inverse)/,
  ],
  ["Graphic marks", /^--ui-chart/],
];

// Text pairs need 4.5:1 (WCAG AA); graphic marks without text need 3:1.
const pairs: [string, string, number][] = [
  ["--ui-text", "--ui-surface", 4.5],
  ["--ui-text", "--ui-canvas", 4.5],
  ["--ui-muted", "--ui-surface", 4.5],
  ["--ui-muted", "--ui-canvas", 4.5],
  ["--ui-muted", "--ui-hover", 4.5],
  ["--ui-faint", "--ui-surface", 4.5],
  ["--ui-faint", "--ui-canvas", 4.5],
  ["--ui-primary", "--ui-surface", 4.5],
  ["--ui-primary", "--ui-primary-soft", 4.5],
  ["--ui-on-primary", "--ui-primary", 4.5],
  ["--ui-on-primary", "--ui-primary-hover", 4.5],
  ["--ui-success", "--ui-success-soft", 4.5],
  ["--ui-warning", "--ui-warning-soft", 4.5],
  ["--ui-danger", "--ui-danger-soft", 4.5],
  ["--ui-success", "--ui-surface", 4.5],
  ["--ui-warning", "--ui-surface", 4.5],
  ["--ui-danger", "--ui-surface", 4.5],
  ["--ui-success", "--ui-canvas", 4.5],
  ["--ui-warning", "--ui-canvas", 4.5],
  ["--ui-danger", "--ui-canvas", 4.5],
  ["--ui-info", "--ui-info-soft", 4.5],
  ["--ui-neutral", "--ui-neutral-soft", 4.5],
  ["--ui-on-inverse", "--ui-inverse", 4.5],
  ["--ui-inverse-muted", "--ui-inverse", 4.5],
  ["--ui-control-border", "--ui-surface", 3],
  ["--ui-control-border", "--ui-canvas", 3],
  ["--ui-control-checked", "--ui-surface", 3],
  ["--ui-on-control-checked", "--ui-control-checked", 3],
  ["--ui-sidebar-text", "--ui-sidebar", 4.5],
  ["--ui-sidebar-muted", "--ui-sidebar", 4.5],
  ["--ui-sidebar-active-text", "--ui-sidebar-active", 4.5],
  ["--ui-sidebar-active-text", "--ui-sidebar-hover", 4.5],
  ["--ui-focus", "--ui-surface", 3],
  ["--ui-chart-success", "--ui-surface", 3],
  ["--ui-chart-warning", "--ui-surface", 3],
  ["--ui-chart-danger", "--ui-surface", 3],
];

function Swatch({ value }: { value: string }) {
  return (
    <span className="flex items-center gap-2">
      <span
        className="inline-block size-7 shrink-0 rounded-md border border-line"
        style={{ background: value }}
      />
      <code>{value}</code>
    </span>
  );
}

function Ratio({ fg, bg, min }: { fg: string; bg: string; min: number }) {
  const ratio = contrast(fg, bg);
  if (ratio === null) return <span className="text-muted">n/a</span>;
  const pass = ratio >= min;
  return (
    <span className="flex items-center gap-2">
      {min < 4.5 ? (
        // A 3:1 pair is a mark or boundary, never text.
        <span
          className="inline-flex h-7 min-w-14 items-center justify-center rounded-md"
          style={{ background: bg }}
        >
          <span
            className="inline-block size-4 rounded-sm"
            style={{ border: `2px solid ${fg}` }}
          />
        </span>
      ) : (
        <span
          className="inline-flex h-7 min-w-14 items-center justify-center rounded-md px-2 font-semibold"
          style={{ color: fg, background: bg }}
        >
          Aa
        </span>
      )}
      <strong className={pass ? "text-success" : "text-danger"}>
        {ratio.toFixed(2)}:1 {pass ? "pass" : "FAIL"}
      </strong>
    </span>
  );
}

function ColorTokens() {
  const names = Object.keys(light).filter((name) => isColor(light[name]));
  return (
    <div className="grid gap-6">
      <p className="text-muted">
        Signal is the only theme. Values are read from <code>themes.css</code>;
        both modes are shown side by side whatever the toolbar mode. Inside{" "}
        <code>.sidebar</code> the generic text and surface tokens resolve to the
        ink palette.
      </p>
      {groups.map(([title, match]) => (
        <section key={title} className="panel p-5">
          <h2 className="mb-3">{title}</h2>
          <table className="w-full text-left">
            <thead>
              <tr>
                <th>Token</th>
                <th>Light</th>
                <th>Dark</th>
                {title === "Status" || title === "Graphic marks" ? (
                  <th>L* light / dark</th>
                ) : null}
              </tr>
            </thead>
            <tbody>
              {names
                .filter((name) => match.test(name))
                .map((name) => (
                  <tr key={name}>
                    <td>
                      <code>{name}</code>
                    </td>
                    <td>
                      <Swatch value={light[name]} />
                    </td>
                    <td>
                      <Swatch value={dark[name]} />
                    </td>
                    {title === "Status" || title === "Graphic marks" ? (
                      <td>
                        {[light[name], dark[name]]
                          .map((v) => lightness(v)?.toFixed(0) ?? "n/a")
                          .join(" / ")}
                      </td>
                    ) : null}
                  </tr>
                ))}
            </tbody>
          </table>
        </section>
      ))}
    </div>
  );
}

function ContrastPairs() {
  return (
    <section className="panel p-5">
      <h2 className="mb-1">Contrast</h2>
      <p className="mb-4 text-muted">
        Text pairs need 4.5:1 (WCAG AA); focus rings and marks without text need
        3:1.
      </p>
      <table className="w-full text-left">
        <thead>
          <tr>
            <th>Foreground on background</th>
            <th>Light</th>
            <th>Dark</th>
          </tr>
        </thead>
        <tbody>
          {pairs.map(([fg, bg, min]) => (
            <tr key={fg + bg}>
              <td>
                <code>{fg}</code> on <code>{bg}</code> (≥ {min}:1)
              </td>
              <td>
                <Ratio fg={light[fg]} bg={light[bg]} min={min} />
              </td>
              <td>
                <Ratio fg={dark[fg]} bg={dark[bg]} min={min} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

const meta = {
  title: "Foundations/Colors",
  parameters: { layout: "padded" },
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Tokens: Story = { render: () => <ColorTokens /> };
export const Contrast: Story = { render: () => <ContrastPairs /> };
