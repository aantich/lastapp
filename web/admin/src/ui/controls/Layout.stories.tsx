import type { Meta, StoryObj } from "@storybook/react-vite";
import { Card, CardBody, CardHeader } from "../card";
import { Inline, Split, Stack } from "../layout";
import { Badge, StatusBadge } from "../status";
import { Button } from "../button";

// layout.tsx: a screen's vertical rhythm (Stack), wrapping rows (Inline) and
// a record list beside the open record (Split; stacks under 800 px).
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
