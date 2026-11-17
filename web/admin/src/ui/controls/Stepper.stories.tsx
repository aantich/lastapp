import type { Meta, StoryObj } from "@storybook/react-vite";
import { Avatar } from "../avatar";
import { StatusBadge } from "../status";
import { Progress, Stepper, Timeline } from "../stepper";

// stepper.tsx: deploy/publish pipelines, determinate progress and history.
const meta = { title: "Primitives/Stepper" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const DeployInProgress: Story = {
  render: () => (
    <div className="grid gap-4">
      <Stepper
        label="Deploy r15"
        steps={[
          {
            id: "queued",
            label: "Queued",
            note: "09:41 by Ava Chen",
            state: "done",
          },
          {
            id: "storage",
            label: "Preparing storage",
            note: "Deal, Account",
            state: "done",
          },
          {
            id: "publishing",
            label: "Publishing",
            note: "Uploading 14 files",
            state: "active",
          },
          { id: "active", label: "Active", state: "pending" },
        ]}
      />
      <Progress value={3} max={4} label="Step 3 of 4 · Publishing" />
    </div>
  ),
};

export const DeployFailed: Story = {
  render: () => (
    <Stepper
      label="Deploy r14"
      steps={[
        { id: "queued", label: "Queued", state: "done" },
        {
          id: "storage",
          label: "Preparing storage",
          note: "Deal changed during publication",
          state: "failed",
        },
        { id: "publishing", label: "Publishing", state: "pending" },
        { id: "active", label: "Active", state: "pending" },
      ]}
    />
  ),
};

export const Publish: Story = {
  render: () => (
    <Stepper
      label="Publish Invoice"
      steps={[
        { id: "drafted", label: "Drafted", state: "done" },
        { id: "validated", label: "Validated", state: "done" },
        { id: "review", label: "Review", note: "3 changes", state: "active" },
        { id: "storage", label: "Prepare storage", state: "pending" },
        { id: "published", label: "Published", state: "pending" },
      ]}
    />
  ),
};

export const History: Story = {
  render: () => (
    <div className="panel max-w-xl p-5">
      <Timeline
        label="Release history"
        events={[
          {
            id: "3",
            marker: <Avatar name="Claude Desktop" kind="agent" />,
            title: "Deployed r15",
            meta: <StatusBadge state="publishing" />,
            detail: "On behalf of Ava Chen",
            at: "2026-11-18T09:41:00Z",
            when: "Today, 09:41",
          },
          {
            id: "2",
            marker: <Avatar name="Mateo Alvarez" />,
            title: "Deploy r14 failed",
            meta: <StatusBadge state="failed" />,
            detail: "Storage preparation stopped at Deal",
            at: "2026-11-17T16:02:00Z",
            when: "Yesterday, 16:02",
          },
          {
            id: "1",
            marker: <Avatar name="Worker" kind="system" />,
            title: "Activated r13",
            meta: <StatusBadge state="active" />,
            at: "2026-11-12T11:20:00Z",
            when: "12 Nov, 11:20",
          },
        ]}
      />
    </div>
  ),
};
