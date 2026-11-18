import type { Meta, StoryObj } from "@storybook/react-vite";
import { RecordList } from "../record-list";
import { Section, Lead } from "../section";
import {
  Card,
  CardBody,
  CardHeader,
  AttentionCard,
  AttentionGrid,
} from "../card";
import { Avatar } from "../avatar";
import { StatusBadge } from "../status";
import { Button } from "../button";
import { Stack } from "../layout";

// record-list.tsx: compact name + meta rows inside a card (Home's recent Apps
// and connected agents); the `detail` state hides under 800 px. section.tsx:
// a titled page part outside a card, and the page's Lead line.
const meta = { title: "Composites/Record list" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const apps = [
  ["Sales Desk", "CRM", "active", "active"],
  ["Dispatch Board", "Field Service", "failed", "active"],
  ["Spend Approvals", "Finance Ops", null, "active"],
  [
    "Partner Onboarding with a deliberately long name",
    "Partner Portal",
    "publishing",
    "archived",
  ],
] as const;

export const RecentApps: Story = {
  render: () => (
    <Card>
      <CardHeader
        title="Apps"
        description="Most recently created"
        actions={<Button variant="link">All Apps</Button>}
      />
      <CardBody>
        <RecordList
          label="Recent Apps"
          rows={apps.map(([name, space, deploy, state]) => ({
            id: name,
            leading: <Avatar name={name} size={34} />,
            title: name,
            meta: space,
            detail: deploy && (
              <StatusBadge state={deploy} label={`Deploy ${deploy}`} />
            ),
            trailing: (
              <>
                <StatusBadge state={state} />
                <Button size="sm" variant="ghost" aria-label={`Open ${name}`}>
                  Open
                </Button>
              </>
            ),
          }))}
        />
      </CardBody>
    </Card>
  ),
};

export const ConnectedAgents: Story = {
  render: () => (
    <Card>
      <CardHeader title="Agents" description="Connected in this Tenant" />
      <CardBody>
        <RecordList
          label="Connected agents"
          rows={[
            ["Claude Code", "Ava Chen", "CRM", "Used 4 minutes ago"],
            ["Release bot", "Mateo Alvarez", "Field Service", "Not used yet"],
          ].map(([client, owner, scope, used]) => ({
            id: client,
            leading: <Avatar name={client} kind="agent" size={34} />,
            title: client,
            meta: `for ${owner} · ${scope}`,
            trailing: <small>{used}</small>,
          }))}
        />
      </CardBody>
    </Card>
  ),
};

export const SectionWithLead: Story = {
  render: () => (
    <Stack>
      <Lead>Good morning, Ava. 1 thing needs you in Northstar.</Lead>
      <Section title="Needs attention">
        <AttentionGrid>
          <AttentionCard
            state="failed"
            meta="2 minutes ago"
            title="Deploy of Dispatch Board failed"
            actions={
              <Button size="sm" variant="primary">
                Open releases
              </Button>
            }
          >
            The deployment did not finish.
          </AttentionCard>
        </AttentionGrid>
      </Section>
    </Stack>
  ),
};
