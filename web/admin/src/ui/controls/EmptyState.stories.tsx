import type { Meta, StoryObj } from "@storybook/react-vite";
import { Bot, Plus } from "lucide-react";
import { Button } from "../button";
import { EmptyState } from "../empty-state";

const meta = { title: "Primitives/EmptyState" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithAction: Story = {
  render: () => (
    <div className="panel">
      <EmptyState
        title="No Apps in Field Service yet"
        actions={
          <Button variant="primary" icon={<Plus size={16} aria-hidden />}>
            Create App
          </Button>
        }
      >
        An App reuses this Space's entities and may narrow who can see them.
      </EmptyState>
    </div>
  ),
};

export const Compact: Story = {
  render: () => (
    <div className="panel">
      <EmptyState
        compact
        icon={<Bot size={28} aria-hidden />}
        title="No agent access"
      >
        Connect an agent from its MCP client to see it here.
      </EmptyState>
    </div>
  ),
};

export const TextOnly: Story = {
  render: () => (
    <div className="panel">
      <EmptyState>No records yet.</EmptyState>
    </div>
  ),
};
