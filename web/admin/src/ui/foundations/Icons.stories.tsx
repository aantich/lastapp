import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { icons } from "lucide-react";
import { Icon } from "../icon";

// The lucide-react icons the console imports today, found in its source.
const sources = import.meta.glob<string>(
  ["../../**/*.tsx", "!../../**/*.stories.tsx"],
  { query: "?raw", import: "default", eager: true },
);
const used = [
  ...new Set(
    Object.values(sources).flatMap((code) =>
      [...code.matchAll(/import\s*\{([^}]*)\}\s*from\s*"lucide-react"/g)]
        .flatMap(([, list]) => list.split(","))
        .map((name) => name.trim())
        .filter((name) => name && !name.startsWith("type ")),
    ),
  ),
]
  .filter((name): name is keyof typeof icons => name in icons)
  .sort();

function Icons() {
  const [query, setQuery] = useState("");
  const shown = used.filter((name) =>
    name.toLowerCase().includes(query.toLowerCase()),
  );
  return (
    <section className="panel p-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2>Icons in use ({used.length})</h2>
        <label className="search">
          <input
            aria-label="Filter icons"
            placeholder="Filter icons…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
      </div>
      <p className="mb-4 text-muted">
        lucide-react, 1.7 stroke in navigation, sized 15–21px beside text. Add
        new icons from the same set.
      </p>
      <ul className="grid grid-cols-3 gap-3 sm:grid-cols-6">
        {shown.map((name) => {
          const Glyph = icons[name];
          return (
            <li
              key={name}
              className="grid justify-items-center gap-2 rounded-panel border border-line p-3 text-center"
            >
              <Icon icon={Glyph} size="3xl" aria-hidden="true" />
              <code>{name}</code>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

const meta = { title: "Foundations/Icons" } satisfies Meta;
export default meta;
export const InUse: StoryObj<typeof meta> = { render: () => <Icons /> };
