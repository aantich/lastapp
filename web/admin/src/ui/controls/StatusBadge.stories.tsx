import type { Meta, StoryObj } from "@storybook/react-vite";
import { Badge, StatusBadge } from "../status";
import { statusTone } from "../status-tone";

// status.tsx: StatusBadge reads the one state → tone map (status-tone.ts);
// each tone has its own mark shape. Badge is a passive tag or count.
const states = [
  "active",
  "ready",
  "published",
  "prepared",
  "queued",
  "pending",
  "provisioning",
  "preparing",
  "publishing",
  "suspended",
  "draft",
  "failed",
  "disabled",
  "revoked",
  "archived",
  "retired",
  "rotated",
  "something_new",
];

const meta = {
  title: "Primitives/StatusBadge",
  component: StatusBadge,
  args: { state: "active" },
} satisfies Meta<typeof StatusBadge>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Playground: Story = {};

export const AllStates: Story = {
  render: () => (
    <table className="w-full max-w-xl text-left">
      <thead>
        <tr>
          <th>State</th>
          <th>Tone</th>
          <th>Badge</th>
        </tr>
      </thead>
      <tbody>
        {states.map((state) => (
          <tr key={state}>
            <td>
              <code>{state}</code>
            </td>
            <td>{statusTone(state)}</td>
            <td>
              <StatusBadge state={state} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  ),
};

export const CustomLabels: Story = {
  render: () => (
    <div className="flex flex-wrap gap-2">
      <StatusBadge state="active" label="Live" />
      <StatusBadge state="publishing" label="Deploying r15" />
      <StatusBadge state="failed" label="Deploy failed" />
      <StatusBadge state="draft" label="Draft · 3 changes" />
      <StatusBadge state="archived" label="Previous" />
    </div>
  ),
};

export const Tags: Story = {
  render: () => (
    <div className="grid gap-3">
      <div className="flex flex-wrap gap-2">
        <Badge>read</Badge>
        <Badge>acl.manage</Badge>
        <Badge mono>app_crm_0001</Badge>
        <Badge mono>deal.amount</Badge>
      </div>
      <div className="flex flex-wrap gap-2">
        <Badge tone="info">new</Badge>
        <Badge tone="success">prod</Badge>
        <Badge tone="warning">dev</Badge>
        <Badge tone="danger">lost</Badge>
        <Badge tone="inverse">Space admin</Badge>
      </div>
      <div className="flex flex-wrap gap-2">
        <Badge count>3</Badge>
        <Badge count tone="info">
          12
        </Badge>
      </div>
    </div>
  ),
};
