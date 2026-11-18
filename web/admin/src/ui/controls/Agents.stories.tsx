import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  ChoiceCards,
  ConnectionCard,
  ConnectionGrid,
  FlowCard,
  FlowSection,
} from "../connection-card";
import { Badge, StatusBadge } from "../status";
import { Button } from "../button";
import { SegmentedControl } from "../segmented";

// connection-card.tsx: agent connections (agent-access.tsx) and the consent
// card. Neutral sample data; the real screens read grants and the agent
// config.
function Connections() {
  return (
    <ConnectionGrid>
      {[
        { name: "Sample agent", active: true },
        { name: "Other client", active: false },
      ].map((g) => (
        <ConnectionCard
          key={g.name}
          name={g.name}
          subtitle="for Alex Kim · Space Sample"
          status={<StatusBadge state={g.active ? "active" : "revoked"} />}
          inactive={!g.active}
          chips={
            <>
              <Badge pill tone="info">
                Inspect only
              </Badge>
              <Badge pill mono>
                read
              </Badge>
            </>
          }
          meta={
            <>
              <span>Expires in 6 days</span>
              <span aria-hidden>·</span>
              <span>never used</span>
            </>
          }
          actions={
            g.active && (
              <Button size="sm" variant="danger">
                Revoke
              </Button>
            )
          }
        />
      ))}
    </ConnectionGrid>
  );
}

function Consent() {
  const [preset, setPreset] = useState("a"),
    [days, setDays] = useState("7");
  return (
    <FlowCard
      title="Connect an agent"
      footer={
        <>
          <Button>Cancel</Button>
          <Button variant="primary">Approve connection</Button>
        </>
      }
    >
      <FlowSection label="Agent">
        <strong>Sample agent</strong>
      </FlowSection>
      <FlowSection label="What may it do?">
        <ChoiceCards
          label="Permission presets"
          value={preset}
          onChange={setPreset}
          options={[
            { value: "a", label: "Inspect only", description: "Read only" },
            { value: "b", label: "Author", description: "Draft and check" },
            {
              value: "c",
              label: "Publisher",
              description: "Not requested",
              disabled: true,
            },
          ]}
        />
      </FlowSection>
      <FlowSection label="Expires after">
        <SegmentedControl
          label="Access length"
          value={days}
          onChange={setDays}
          options={["1", "7", "30"].map((d) => ({
            value: d,
            label: d === "1" ? "1 day" : d + " days",
          }))}
        />
      </FlowSection>
      <p>Explanatory note under the choices.</p>
    </FlowCard>
  );
}

const meta = { title: "Patterns/Agents" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const ConnectionCards: Story = { render: () => <Connections /> };
export const ConsentCard: Story = { render: () => <Consent /> };
