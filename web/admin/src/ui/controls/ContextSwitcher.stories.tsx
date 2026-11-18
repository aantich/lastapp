import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ContextSwitcher, type ContextOption } from "../context-switcher";
import { spaces } from "../storybook/fixtures";
import { mockApi } from "../storybook/mock-api";
import { ConsoleStory } from "../storybook/console-story";
import { policy } from "../storybook/fixtures";
import { text, type Item } from "../../api-types";

// context-switcher.tsx: the Tenant/Space searchable select (a filter
// combobox over a listbox in the kit's floating ListboxPopup; custom, never
// a native select). The popup is portalled over the page, so it never moves
// the sidebar or takes its ink colours.
const spaceOptions: ContextOption[] = spaces.map((s) => ({
  value: text(s.id),
  label: text(s.display_name),
  description: text(s.description),
  disabled: s.state === "archived",
}));

// The paged lists the console's switchers read (served by mockApi, 5 per
// page, so "Load more" appears).
const spacesPath = "/v1/tenants/ten_northstar/spaces";
const tenantsPath = "/v1/my-tenants";
const tenants: Item[] = Array.from({ length: 12 }, (_, i) => ({
  id: `ten_${String(i + 1).padStart(2, "0")}`,
  display_name: `Example Tenant ${i + 1}`,
}));

type Args = {
  variant?: "field" | "sidebar";
  ink?: boolean;
  rail?: boolean;
  paged?: boolean;
};

function Demo({ variant = "field", ink, rail, paged }: Args) {
  const [space, setSpace] = useState("spc_0001"),
    [tenant, setTenant] = useState("ten_01");
  const sidebar = variant === "sidebar";
  const switcher = sidebar ? (
    <ContextSwitcher
      kind="Tenant"
      variant="sidebar"
      rail={rail}
      value={tenant}
      current="Example Tenant 1"
      options={paged ? [] : tenants.slice(0, 3).map(toOption)}
      path={paged ? tenantsPath : undefined}
      onChange={setTenant}
    />
  ) : paged ? (
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
      {ink || sidebar ? (
        <div className={rail ? "sidebar-collapsed" : undefined}>
          <div
            className="sidebar"
            // Static, so the drawer rules of narrow screens do not hide it.
            style={{
              position: "static",
              height: 460,
              transform: "none",
              visibility: "visible",
            }}
          >
            {switcher}
          </div>
        </div>
      ) : (
        <div className="max-w-xs" style={{ minHeight: 420 }}>
          {switcher}
        </div>
      )}
    </ConsoleStory>
  );
}

const toOption = (t: Item): ContextOption => ({
  value: text(t.id),
  label: text(t.display_name),
});

// Opens the switcher (and types a search) once the story renders.
function openSwitcher(search?: string) {
  return async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    canvasElement
      .querySelector<HTMLButtonElement>("button[aria-haspopup=listbox]")
      ?.click();
    if (!search) return;
    await new Promise((r) => setTimeout(r, 50));
    const input = document.querySelector<HTMLInputElement>(
      ".ui-context-popup input",
    );
    if (!input) return;
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )?.set?.call(input, search);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  };
}

const meta = {
  title: "Composites/ContextSwitcher",
  component: Demo,
} satisfies Meta<typeof Demo>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OnSurface: Story = {};
export const OnInkSidebar: Story = { args: { ink: true } };
// Server search and "Load more" over a list API.
export const Paged: Story = {
  args: { paged: true },
  beforeEach: () => mockApi({ [spacesPath]: spaces }),
};
// The sidebar's Tenant switcher under the brand row.
export const SidebarTenant: Story = { args: { variant: "sidebar" } };
// Open: the floating popup under the trigger, the current Tenant checked.
export const SidebarOpen: Story = {
  args: { variant: "sidebar" },
  play: openSwitcher(),
};
// A long paged list: further pages load inside the popup.
export const SidebarLoadMore: Story = {
  args: { variant: "sidebar", paged: true },
  beforeEach: () => mockApi({ [tenantsPath]: tenants }),
  play: openSwitcher(),
};
// A search with no results.
export const SidebarNoResults: Story = {
  args: { variant: "sidebar" },
  play: openSwitcher("zzz"),
};
// The collapsed 68 px rail: the initials tile alone; the popup opens beside it.
export const SidebarRail: Story = {
  args: { variant: "sidebar", rail: true },
  play: openSwitcher(),
};
