import type { Meta, StoryObj } from "@storybook/react-vite";
import { ActivityList, type ActivityItem } from "../activity-list";
import { Button } from "../button";
import {
  AllClear,
  AppCard,
  AttentionCard,
  AttentionGrid,
  StatCard,
  StatGrid,
} from "../card";
import { PageIntro } from "../page-header";
import { Progress } from "../stepper";
import { Section } from "../section";
import { StatusBadge } from "../status";

// The Tenant Home (tenant/home-page.tsx) from kit parts: the greeting with
// its stats, what needs a person, the Apps and the latest activity.
const meta = { title: "Patterns/Home" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const activity: ActivityItem[] = [
  {
    id: "1",
    actor: { name: "Claude", kind: "agent" },
    text: "Claude started workspace crm-pricing-exp",
    meta: "for Maya · 8 minutes ago",
  },
  {
    id: "2",
    actor: { name: "System worker", kind: "system" },
    text: "Deploy of Forecasting r14 failed",
    meta: "Worker · 12 minutes ago",
  },
  {
    id: "3",
    actor: { name: "Joe Park", kind: "person" },
    text: "Joe Park deployed Deal review r23",
    meta: "Console · 2 hours ago",
  },
];

function Home({ attention }: { attention: boolean }) {
  return (
    <div className="ui-home">
      <PageIntro
        eyebrow="Tuesday, 17 November"
        title="Good morning, Maya"
        summary={
          attention
            ? "2 things need you. Everything else in Northstar is healthy."
            : "Nothing needs you right now. Everything in Northstar is healthy."
        }
        aside={
          <StatGrid>
            <StatCard label="Entities" value="14" />
            <StatCard label="Apps live" value="5" />
            <StatCard label="Agents active" value="2" />
          </StatGrid>
        }
      />
      <Section title="Needs attention">
        {attention ? (
          <AttentionGrid>
            <AttentionCard
              state="failed"
              stateLabel="Deploy failed"
              meta="Forecasting · 12 minutes ago"
              title="r14 did not go live"
              actions={
                <>
                  <Button size="sm" variant="primary">
                    Retry deploy
                  </Button>
                  <Button size="sm">View timeline</Button>
                </>
              }
            >
              Storage gate conflict while publishing deal v9. Production still
              serves r13.
            </AttentionCard>
            <AttentionCard
              state="deploying"
              meta="Deal review · r24"
              title="Deploying r24"
              extra={
                <Progress
                  value={2}
                  max={4}
                  label="Step 2 of 4 · Preparing storage"
                />
              }
            >
              Production serves r23 until the switch.
            </AttentionCard>
          </AttentionGrid>
        ) : (
          <AllClear>
            No failed or running deploys, databases being prepared or agent
            access about to expire.
          </AllClear>
        )}
      </Section>
      <div className="ui-home-columns">
        <Section
          className="ui-home-apps"
          title="Apps"
          actions={<Button variant="link">All apps →</Button>}
        >
          <div className="ui-app-grid">
            <AppCard
              id="deal-review"
              name="Deal review"
              subtitle="CRM · 4 micro-apps"
              status={<StatusBadge state="active" label="Live" />}
              address="deal-review.northstar.example"
              footer="Production r23 · deployed 2 hours ago by Joe Park"
              onOpen={() => {}}
            />
            <AppCard
              id="forecasting"
              name="Forecasting"
              subtitle="CRM · 2 micro-apps"
              status={<StatusBadge state="failed" label="r14 failed" />}
              address="forecasting.northstar.example"
              footer="Production r13 · deployed yesterday by Maya Chen"
              onOpen={() => {}}
            />
          </div>
        </Section>
        <section
          className="panel ui-activity-card"
          aria-labelledby="story-activity"
        >
          <header className="ui-section-header">
            <h2 id="story-activity">Activity</h2>
            <Button variant="link">Audit trail →</Button>
          </header>
          <ActivityList label="Recent activity" items={activity} />
        </section>
      </div>
    </div>
  );
}

export const NeedsAttention: Story = { render: () => <Home attention /> };
export const AllClearState: Story = {
  name: "All clear",
  render: () => <Home attention={false} />,
};
