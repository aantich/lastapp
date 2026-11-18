import type { Meta, StoryObj } from "@storybook/react-vite";
import { Card, CardBody, CardHeader } from "../card";
import { Columns, Inline, Split, Stack } from "../layout";
import { Badge, StatusBadge } from "../status";
import { Button } from "../button";
import { TextField } from "../field";

// layout.tsx: a screen's vertical rhythm (Stack), wrapping rows (Inline) and
// a record list beside the open record (Split; stacks under 800 px) and
// cards side by side (Columns: wrapping equal columns or fixed weights).
const meta = { title: "Primitives/Layout" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const StackAndInline: Story = {
  render: () => (
    <Stack>
      <Card>
        <CardHeader title="Production deployment" />
        <CardBody>
          <Inline>
            <StatusBadge state="publishing" />
            <Badge mono>r14</Badge>
            <Button size="sm">Refresh</Button>
          </Inline>
        </CardBody>
      </Card>
      <Card>
        <CardHeader title="Uploaded builds" />
      </Card>
    </Stack>
  ),
};

export const SplitView: Story = {
  render: () => (
    <Split
      aside={
        <Card>
          <CardHeader title="Releases" level={3} />
          <CardBody>r14 · r13 · r12</CardBody>
        </Card>
      }
    >
      <Card>
        <CardHeader title="r14 · Forecast v2" level={3} />
        <CardBody>The open release.</CardBody>
      </Card>
    </Split>
  ),
};

export const ColumnsGrid: Story = {
  render: () => (
    <Columns min={240}>
      {["Customers", "Invoices", "Shipments", "Products"].map((name) => (
        <Card key={name}>
          <CardHeader title={name} level={3} />
          <CardBody>Published entity</CardBody>
        </Card>
      ))}
    </Columns>
  ),
};

export const ColumnsWeighted: Story = {
  render: () => (
    <Columns weights="minmax(0, 2fr) minmax(0, 1fr)">
      <Card>
        <CardHeader title="Needs attention" />
        <CardBody>Main column</CardBody>
      </Card>
      <Card>
        <CardHeader title="Agents" />
        <CardBody>Side column</CardBody>
      </Card>
    </Columns>
  ),
};

// Each column keeps its own minimum; they stack as soon as both cannot fit
// (Home: Apps beside Recent activity).
export const ColumnsTracks: Story = {
  render: () => (
    <Columns
      tracks={[
        { grow: 8, min: 480 },
        { grow: 5, min: 320 },
      ]}
    >
      <Card>
        <CardHeader title="Apps" description="Most recently created" />
        <CardBody>Main column</CardBody>
      </Card>
      <Card>
        <CardHeader title="Recent activity" />
        <CardBody>Side column</CardBody>
      </Card>
    </Columns>
  ),
};

// align="end" lines a labelled field up with a button by their bottoms.
export const InlineAlignEnd: Story = {
  render: () => (
    <Inline align="end">
      <TextField label="Name" defaultValue="Deal" />
      <Button variant="primary">Add field</Button>
    </Inline>
  ),
};
