import type { Meta, StoryObj } from "@storybook/react-vite";
import { Info } from "lucide-react";
import { Avatar } from "../avatar";
import { Kbd, Skeleton, Spinner } from "../feedback";
import { Tooltip } from "../tooltip";

// avatar.tsx, feedback.tsx (Spinner, Skeleton, Kbd) and tooltip.tsx.
const meta = { title: "Primitives/Display" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Avatars: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Avatar name="Ava Chen" labelled />
      <Avatar name="Priya Natarajan" size={40} labelled />
      <Avatar name="Omar" size={24} labelled />
      <Avatar name="Claude Desktop" kind="agent" labelled />
      <Avatar name="Worker" kind="system" labelled />
    </div>
  ),
};

export const Keys: Story = {
  render: () => (
    <p className="flex items-center gap-1">
      Open the command palette with <Kbd>⌘</Kbd>
      <Kbd>K</Kbd> or <Kbd>Ctrl</Kbd>
      <Kbd>K</Kbd>.
    </p>
  ),
};

export const Loading: Story = {
  render: () => (
    <div className="grid max-w-md gap-4">
      <Spinner />
      <Spinner label="Loading members" hideLabel />
      <div className="panel grid gap-3 p-4" aria-busy="true">
        <div className="flex items-center gap-3">
          <Skeleton circle width={32} height={32} />
          <div className="grid flex-1 gap-2">
            <Skeleton width="50%" />
            <Skeleton width="80%" />
          </div>
        </div>
        <Skeleton height={60} />
      </div>
    </div>
  ),
};

export const Tooltips: Story = {
  render: () => (
    <div className="flex gap-6 p-10">
      <Tooltip content="Stored calculations run on the server">
        <button type="button" className="secondary">
          <Info size={15} aria-hidden /> Calculated
        </button>
      </Tooltip>
      <Tooltip content="Right side" side="right">
        <button type="button" className="secondary">
          Right
        </button>
      </Tooltip>
    </div>
  ),
};
