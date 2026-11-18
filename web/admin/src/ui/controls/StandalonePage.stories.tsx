import type { Meta, StoryObj } from "@storybook/react-vite";
import { StandalonePage } from "../standalone-page";
import { PageHeader } from "../page-header";
import { Card, CardBody, CardFooter } from "../card";
import { Button } from "../button";

// standalone-page.tsx: a page outside the console shell (the agent consent
// page): canvas background, one centred column, a 16 px gutter on phones.
const meta = {
  title: "Primitives/StandalonePage",
  parameters: { layout: "fullscreen" },
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Consent: Story = {
  render: () => (
    <StandalonePage>
      <PageHeader
        title="Connect an agent"
        description="Choose exactly what this client may do as Maya Chen."
      />
      <Card>
        <CardBody>
          Codex asks to manage the Northstar CRM Space configuration.
        </CardBody>
        <CardFooter>
          <Button>Cancel</Button>
          <Button variant="primary">Approve agent access</Button>
        </CardFooter>
      </Card>
    </StandalonePage>
  ),
};
