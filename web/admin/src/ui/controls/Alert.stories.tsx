import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Alert, InlineError } from "../alert";
import { Button } from "../button";

// alert.tsx: one banner for every screen message. Danger keeps .error and
// role=alert; other tones are role=status.
const meta = { title: "Primitives/Alert" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Tones: Story = {
  render: () => (
    <div className="grid max-w-2xl">
      <Alert>
        The Deal entity changed since you opened it. Reload to see the latest
        revision.
      </Alert>
      <Alert tone="warning" title="Development workspace">
        Changes here never reach production until you deploy a release.
      </Alert>
      <Alert tone="info">
        Release r15 is publishing. Apps keep serving r14 until it is active.
      </Alert>
      <Alert tone="success">Ava Chen now manages the CRM Space.</Alert>
    </div>
  ),
};

function Dismissable() {
  const [shown, setShown] = useState(true);
  return shown ? (
    <Alert
      title="Deploy failed"
      actions={
        <Button size="sm" variant="danger">
          Retry
        </Button>
      }
      onDismiss={() => setShown(false)}
    >
      Storage preparation stopped at the Invoice entity.
    </Alert>
  ) : (
    <Button onClick={() => setShown(true)}>Show again</Button>
  );
}

export const WithActions: Story = { render: () => <Dismissable /> };

export const Stored: Story = {
  render: () => (
    <Alert as="p" live={false} icon={false}>
      worker: lock wait timeout while preparing storage
    </Alert>
  ),
};

export const FieldError: Story = {
  render: () => (
    <div className="grid max-w-sm gap-2">
      <InlineError>
        Slug must use lowercase letters, digits and hyphens.
      </InlineError>
    </div>
  ),
};
