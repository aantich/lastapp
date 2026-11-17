import type { Meta, StoryObj } from "@storybook/react-vite";
import { RefreshCw } from "lucide-react";
import { Button, IconButton } from "../button";
import {
  AttentionCard,
  AttentionGrid,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  StatCard,
  StatGrid,
} from "../card";

// card.tsx: the .panel surface with header/body/footer, KPI tiles and the
// cards that ask a person to act.
const meta = { title: "Primitives/Card" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithParts: Story = {
  render: () => (
    <Card className="max-w-xl">
      <CardHeader
        title="Database"
        description="The CRM Space's MariaDB schema on db-eu-1. The binding cannot change."
        actions={
          <IconButton
            label="Refresh database"
            icon={<RefreshCw size={16} aria-hidden />}
          />
        }
      />
      <CardBody>
        <p>
          Schema <code>crm_prod</code> holds 14 entities and 38,210 records.
        </p>
      </CardBody>
      <CardFooter>
        <Button>View entities</Button>
        <Button variant="primary">Open designer</Button>
      </CardFooter>
    </Card>
  ),
};

export const Stats: Story = {
  render: () => (
    <StatGrid>
      <StatCard label="Spaces" value="7" detail="1 archived" />
      <StatCard label="Apps" value="12" onClick={() => {}} />
      <StatCard label="Members" value="48" detail="3 invited this week" />
      <StatCard label="Agent grants" value="5" onClick={() => {}} />
    </StatGrid>
  ),
};

export const Attention: Story = {
  render: () => (
    <AttentionGrid>
      <AttentionCard
        state="failed"
        stateLabel="Deploy failed"
        meta="CRM · 12 min ago"
        title="Release r14 did not activate"
        actions={
          <>
            <Button size="sm" variant="primary">
              Retry
            </Button>
            <Button size="sm">View log</Button>
          </>
        }
      >
        Storage preparation stopped: the Deal entity changed while r14 was
        publishing.
      </AttentionCard>
      <AttentionCard
        state="draft"
        stateLabel="Review"
        meta="Finance Ops"
        title="Invoice has 3 unpublished changes"
        actions={<Button size="sm">Review changes</Button>}
      >
        Mateo added an approval rule and two fields.
      </AttentionCard>
      <AttentionCard
        state="preparing"
        meta="Support · r9"
        title="Preparing storage for Ticket"
      >
        Step 2 of 4. Apps keep serving r8 until activation.
      </AttentionCard>
    </AttentionGrid>
  ),
};
