import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ContextSwitcher, type ContextOption } from "../context-switcher";
import { spaces } from "../storybook/fixtures";
import { text } from "../../api-types";

// context-switcher.tsx: the Tenant/Space popover (filter combobox over a
// listbox; custom, never a native select).
const spaceOptions: ContextOption[] = spaces.map((s) => ({
  value: text(s.id),
  label: text(s.display_name),
  description: text(s.description),
  disabled: s.state === "archived",
}));

function Demo({ ink }: { ink?: boolean }) {
  const [space, setSpace] = useState("spc_0001");
  const switcher = (
    <ContextSwitcher
      kind="Space"
      value={space}
      options={spaceOptions}
      onChange={setSpace}
    />
  );
  return ink ? (
    <div
      className="sidebar"
      style={{ position: "static", height: 420, padding: 16 }}
    >
      {switcher}
    </div>
  ) : (
    <div className="max-w-xs" style={{ minHeight: 420 }}>
      {switcher}
    </div>
  );
}

const meta = {
  title: "Composites/ContextSwitcher",
  component: Demo,
} satisfies Meta<typeof Demo>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OnSurface: Story = {};
export const OnInkSidebar: Story = { args: { ink: true } };
