import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ContextSwitcher, type ContextOption } from "../context-switcher";
import { spaces } from "../storybook/fixtures";
import { mockApi } from "../storybook/mock-api";
import { ConsoleStory } from "../storybook/console-story";
import { policy } from "../storybook/fixtures";
import { text } from "../../api-types";

// context-switcher.tsx: the Tenant/Space popover (filter combobox over a
// listbox; custom, never a native select).
const spaceOptions: ContextOption[] = spaces.map((s) => ({
  value: text(s.id),
  label: text(s.display_name),
  description: text(s.description),
  disabled: s.state === "archived",
}));

// The paged list the console's Tenant and Space switchers read.
const spacesPath = "/v1/tenants/ten_northstar/spaces";

function Demo({ ink, paged }: { ink?: boolean; paged?: boolean }) {
  const [space, setSpace] = useState("spc_0001");
  const switcher = paged ? (
    <ContextSwitcher
      kind="Space"
      value={space}
      options={[]}
      current="CRM"
      path={spacesPath}
      onChange={setSpace}
    />
  ) : (
    <ContextSwitcher
      kind="Space"
      value={space}
      options={spaceOptions}
      onChange={setSpace}
    />
  );
  // ConsoleStory supplies the query cache the paged list reads through.
  return (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      {ink ? (
        <div
          className="sidebar"
          // Static, so the drawer rules of narrow screens do not hide it.
          style={{
            position: "static",
            height: 420,
            padding: 16,
            transform: "none",
            visibility: "visible",
          }}
        >
          {switcher}
        </div>
      ) : (
        <div className="max-w-xs" style={{ minHeight: 420 }}>
          {switcher}
        </div>
      )}
    </ConsoleStory>
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
// Server search and "Load more" over a list API (served by mockApi, 5 per
// page), the paging RemoteSelect uses.
export const Paged: Story = {
  args: { paged: true },
  beforeEach: () => mockApi({ [spacesPath]: spaces }),
};
