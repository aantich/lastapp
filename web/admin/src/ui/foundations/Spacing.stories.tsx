import type { Meta, StoryObj } from "@storybook/react-vite";
import { isColor, layout, light } from "./tokens";

// Shape and layout tokens from themes.css and the style.css root.
const shape = Object.entries({ ...light, ...layout }).filter(
  ([name, value]) => !isColor(value) && !name.startsWith("--ui-font"),
);

function Spacing() {
  return (
    <section className="panel p-5">
      <h2 className="mb-4">Spacing, radius and elevation</h2>
      <div className="grid gap-4">
        {shape.map(([name, value]) => (
          <div
            key={name}
            className="grid items-center gap-4 sm:grid-cols-[220px_1fr]"
          >
            <span>
              <code>{name}</code>
              <small>{value}</small>
            </span>
            <Preview name={name} />
          </div>
        ))}
      </div>
    </section>
  );
}

function Preview({ name }: { name: string }) {
  const v = `var(${name})`;
  if (name.includes("radius"))
    return (
      <span
        className="block h-12 w-28 border border-line bg-primary-soft"
        style={{ borderRadius: v }}
      />
    );
  if (name.includes("shadow"))
    return (
      <span
        className="block h-12 w-28 rounded-panel bg-surface"
        style={{ boxShadow: v }}
      />
    );
  if (name.includes("card-border"))
    return (
      <span
        className="block h-12 w-28 rounded-panel"
        style={{ border: `1px ${v} var(--ui-border)` }}
      />
    );
  return (
    <span
      className="block h-3 max-w-full rounded-sm bg-primary"
      style={{ width: v }}
    />
  );
}

const meta = { title: "Foundations/Spacing and radius" } satisfies Meta;
export default meta;
export const Tokens: StoryObj<typeof meta> = { render: () => <Spacing /> };
