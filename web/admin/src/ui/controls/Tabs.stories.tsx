import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Tabs, type TabItem } from "../tabs";

// tabs.tsx: a controlled WAI-ARIA tablist (←/→, Home/End, roving tabindex).
const items: TabItem[] = [
  { id: "overview", label: "Overview" },
  { id: "members", label: "Members", count: 48 },
  { id: "teams", label: "Teams", count: 6 },
  { id: "roles", label: "Roles" },
  { id: "billing", label: "Billing", disabled: true },
];

function Demo({ activation }: { activation?: "auto" | "manual" }) {
  const [tab, setTab] = useState("members");
  return (
    <div className="panel">
      <Tabs
        label="Tenant people"
        items={items}
        value={tab}
        onChange={setTab}
        activation={activation}
      >
        <div className="p-5">
          <p>
            Showing the <strong>{tab}</strong> panel.
          </p>
        </div>
      </Tabs>
    </div>
  );
}

const meta = { title: "Primitives/Tabs", component: Demo } satisfies Meta<
  typeof Demo
>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Automatic: Story = {};
export const Manual: Story = { args: { activation: "manual" } };
