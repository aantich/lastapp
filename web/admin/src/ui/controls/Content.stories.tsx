import { useState, type Key } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Card, CardBody, CardHeader } from "../card";
import { CodeBlock, Disclosure, FactList, PickList } from "../content";
import { SegmentedControl } from "../segmented";
import { StatusBadge, Badge } from "../status";
import { Button } from "../button";

// content.tsx and segmented.tsx: record facts, on-demand detail, machine
// text, a record list beside its open record and list views.
const meta = { title: "Primitives/Content" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Facts: Story = {
  render: () => (
    <Card className="max-w-xl">
      <CardHeader title="Database" />
      <CardBody>
        <FactList
          facts={[
            { label: "Connection", value: <StatusBadge state="ready" /> },
            { label: "Layout", value: "shared" },
            { label: "Database", value: "ns_crm_7f3a", mono: true },
          ]}
        />
      </CardBody>
    </Card>
  ),
};

export const Disclosures: Story = {
  render: () => (
    <Card className="max-w-xl">
      <CardBody>
        <Disclosure summary="Deal" meta="9 fields · 4 permissions">
          <FactList
            facts={[
              { label: "Name", value: "text · Required" },
              { label: "Amount", value: "decimal" },
              { label: "Stage", value: "choice · Required" },
            ]}
          />
        </Disclosure>
        <Disclosure summary="Manifest and full configuration">
          <CodeBlock label="Release r14 configuration">
            {JSON.stringify(
              {
                microapps: { pipeline: "1.4.0", forecast: "0.9.2" },
                dependencies: { react: "19.1.0" },
              },
              null,
              2,
            )}
          </CodeBlock>
        </Disclosure>
      </CardBody>
    </Card>
  ),
};

const releases = [
  {
    id: "r14",
    title: "r14 · Forecast v2",
    meta: "18 Nov 2026, 09:41",
    badge: <StatusBadge state="failed" />,
  },
  {
    id: "r13",
    title: "r13 · Pipeline polish",
    meta: "12 Nov 2026, 16:02",
    badge: <StatusBadge state="active" label="Production" />,
  },
  { id: "r12", title: "r12 · First release", meta: "2 Nov 2026, 11:20" },
];

export const ReleaseHistory: Story = {
  render: function Render() {
    const [open, setOpen] = useState<Key>("r13");
    return (
      <Card className="max-w-sm">
        <CardHeader title="Release history" level={3} />
        <CardBody>
          <PickList
            label="Releases"
            items={releases}
            value={open}
            onPick={setOpen}
            footer={<Button size="sm">Load more</Button>}
          />
        </CardBody>
      </Card>
    );
  },
};

export const EmptyPickList: Story = {
  render: () => (
    <PickList
      label="Releases"
      items={[]}
      onPick={() => {}}
      empty="No releases yet."
    />
  ),
};

export const Segmented: Story = {
  render: function Render() {
    const [actor, setActor] = useState("all");
    return (
      <div className="grid gap-3">
        <SegmentedControl
          label="Actor"
          value={actor}
          onChange={setActor}
          options={[
            { value: "all", label: "All" },
            { value: "person", label: "People" },
            { value: "agent", label: "Agents" },
            { value: "system", label: "System" },
          ]}
        />
        <Badge mono>actor={actor}</Badge>
      </div>
    );
  },
};
