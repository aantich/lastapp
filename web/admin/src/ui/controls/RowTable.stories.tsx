import type { Meta, StoryObj } from "@storybook/react-vite";
import { ExternalLink } from "lucide-react";
import { CellActions, CellMono, CellTitle, RowTable } from "../row-table";
import {
  AppHeader,
  AppTile,
  CardRow,
  PageStack,
  TitledCard,
  ValueBox,
} from "../app-header";
import { Badge, StatusBadge } from "../status";
import { Button, LinkButton } from "../button";
import { Stepper } from "../stepper";
import { Tabs } from "../tabs";
import { Inline } from "../layout";
import { Icon } from "../icon";

// row-table.tsx: a card of fixed-track grid rows (the Apps list, an App's
// releases and micro-apps). app-header.tsx: the App page's header, titled
// cards, side-by-side card rows and the page's vertical rhythm.
const meta = { title: "Composites/Row table and App page" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const apps = [
  ["Sales Desk", "CRM", "success", "Live", "sales-desk.apps.example.test"],
  ["Dispatch Board", "Field Service", "danger", "Build 7 failed", ""],
  ["Spend Approvals", "Finance Ops", "warning", "Deploying", ""],
  ["Partner Onboarding", "Partner Portal", "neutral", "Not deployed", ""],
] as const;

export const AppsList: Story = {
  render: () => (
    <RowTable
      label="Apps"
      columns="minmax(var(--ui-col-name), 2fr) var(--ui-col-label) var(--ui-col-status) minmax(var(--ui-measure-sm), 2fr)"
      headers={["App", "Space", "Status", "Hostname"]}
      rows={apps.map(([name, space, tone, label, host]) => ({
        id: name,
        onOpen: () => {},
        cells: [
          <>
            <AppTile name={name} tone={tone} />
            <strong>{name}</strong>
          </>,
          space,
          <StatusBadge
            state={tone === "success" ? "active" : "draft"}
            tone={tone}
            label={label}
          />,
          <CellMono>{host}</CellMono>,
        ],
      }))}
      footer={
        <>
          <Button size="sm" disabled>
            Previous
          </Button>
          <Button size="sm">Next</Button>
        </>
      }
    />
  ),
};

export const AppPage: Story = {
  render: () => (
    <PageStack>
      <AppHeader
        name="Sales Desk"
        tone="success"
        address="sales-desk.apps.example.test"
        status={<StatusBadge state="active" label="Live" />}
        actions={
          <LinkButton
            href="#"
            variant="primary"
            trailingIcon={<Icon icon={ExternalLink} size="sm" aria-hidden />}
          >
            Launch app
          </LinkButton>
        }
      />
      <Tabs
        label="Sales Desk sections"
        variant="line"
        items={[
          { id: "releases", label: "Releases" },
          { id: "microapps", label: "Micro-apps" },
          { id: "data", label: "Data access" },
        ]}
        value="releases"
        onChange={() => {}}
      />
      <TitledCard
        title="Production deployment"
        note="Build 8 failed. Production still serves Build 7."
        actions={
          <Button size="sm" variant="primary">
            Retry deploy
          </Button>
        }
      >
        <Stepper
          label="Deployment progress"
          min="xs"
          steps={[
            {
              id: "q",
              label: "Queued",
              note: "Release accepted",
              state: "done",
            },
            {
              id: "p",
              label: "Preparing storage",
              note: "A sample storage error",
              state: "failed",
            },
            { id: "u", label: "Publishing", state: "pending" },
            { id: "a", label: "Active", state: "pending" },
          ]}
        />
      </TitledCard>
      <RowTable
        label="Releases"
        title="Releases"
        note="Immutable build + micro-app pins + Space snapshot"
        columns="minmax(var(--ui-measure-sm), 2fr) var(--ui-col-time) auto"
        rows={[
          ["Build 8", "5 minutes ago · Ava Chen", "Failed", "danger"],
          [
            "Build 7",
            "2 days ago · Agent for Ava Chen",
            "Production",
            "success",
          ],
        ].map(([name, meta, chip, tone]) => ({
          id: name,
          cells: [
            <CellTitle title={name} meta={meta} />,
            <StatusBadge
              state="draft"
              tone={tone as "danger" | "success"}
              label={chip}
            />,
            <CellActions>
              <Button size="sm">Details</Button>
            </CellActions>,
          ],
        }))}
      />
      <CardRow>
        <TitledCard
          layout="stacked"
          title="Approved origins"
          note="Browsers allowed to sign in to this App’s data API."
        >
          <ValueBox>https://sales-desk.apps.example.test</ValueBox>
        </TitledCard>
        <TitledCard
          layout="stacked"
          title="Permission ceiling"
          note="The App can narrow Space access, never widen it."
        >
          <Inline>
            <Badge pill>Ticket · Read · title, status</Badge>
            <Badge pill>Customer · Read · name</Badge>
          </Inline>
        </TitledCard>
      </CardRow>
    </PageStack>
  ),
};
