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
// its stats, what needs a person, the Apps and the latest activity. The
// content is neutral sample data; the real page derives every value.
const meta = { title: "Patterns/Home" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const activity: ActivityItem[] = [
  {
    id: "1",
    actor: { name: "Agent", kind: "agent" },
    text: "An agent started workspace sales-exp",
    meta: "for Alex Kim · 8 minutes ago",
  },
  {
    id: "2",
    actor: { name: "System worker", kind: "system" },
    text: "Deploy of Planning 1.1.0 failed",
    meta: "Worker · 12 minutes ago",
  },
  {
    id: "3",
    actor: { name: "Sam Lee", kind: "person" },
    text: "Sam Lee deployed Orders 2.0.0",
    meta: "Console · 2 hours ago",
  },
];

function Home({ attention }: { attention: boolean }) {
  return (
    <div className="ui-home">
      <PageIntro
        eyebrow="Monday, 2 March"
        title="Good morning, Alex"
        summary={
          attention
            ? "2 things need you. Everything else in Example Co is healthy."
            : "Nothing needs you right now. Everything in Example Co is healthy."
        }
        aside={
          <StatGrid>
            <StatCard label="Entities" value="6" />
            <StatCard label="Apps live" value="2" />
            <StatCard label="Agents active" value="1" />
          </StatGrid>
        }
      />
      <Section title="Needs attention">
        {attention ? (
          <AttentionGrid>
            <AttentionCard
              state="failed"
              stateLabel="Deploy failed"
              meta="Planning · 12 minutes ago"
              title="1.1.0 did not go live"
              actions={
                <>
                  <Button size="sm" variant="primary">
                    Retry deploy
                  </Button>
                  <Button size="sm">View timeline</Button>
                </>
              }
            >
              The storage check rejected the release. Production still serves
              1.0.0.
            </AttentionCard>
            <AttentionCard
              state="deploying"
              meta="Orders · 2.1.0"
              title="Deploying 2.1.0"
              extra={
                <Progress
                  value={2}
                  max={4}
                  label="Step 2 of 4 · Preparing storage"
                />
              }
            >
              Production serves 2.0.0 until the switch.
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
              id="orders"
              name="Orders"
              subtitle="Sales · 3 micro-apps"
              status={<StatusBadge state="active" label="Live" />}
              address="orders.example.test"
              footer="Production 2.0.0 · deployed 2 hours ago by Sam Lee"
              onOpen={() => {}}
            />
            <AppCard
              id="planning"
              name="Planning"
              subtitle="Sales · 1 micro-app"
              status={<StatusBadge state="failed" label="1.1.0 failed" />}
              address="planning.example.test"
              footer="Production 1.0.0 · deployed yesterday by Alex Kim"
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

// The Activity page's rows: mark, sentence, via and when as columns
// (stacked under 720 px).
export const ActivityRows: Story = {
  name: "Activity rows",
  render: () => (
    <div className="panel">
      <ActivityList
        label="Activity"
        variant="rows"
        items={activity.map((a) => {
          const [via, when] = String(a.meta).split(" · ");
          return { ...a, via, when };
        })}
      />
    </div>
  ),
};
