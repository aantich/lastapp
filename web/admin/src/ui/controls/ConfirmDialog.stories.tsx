import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ConfirmDialog } from "../confirm-dialog";
import type { ConfirmOptions } from "../confirm";

// confirm-dialog.tsx renders confirmAction() requests (confirm.ts); screens
// never use window.confirm. Danger dialogs focus Cancel first.
function Confirm(props: ConfirmOptions) {
  const [open, setOpen] = useState(true),
    [result, setResult] = useState("");
  return (
    <div className="grid justify-items-start gap-3">
      <button className="secondary" onClick={() => setOpen(true)}>
        Open dialog
      </button>
      {result && <p role="status">Result: {result}</p>}
      {open && (
        <ConfirmDialog
          {...props}
          onResult={(confirmed) => {
            setResult(confirmed ? "confirmed" : "cancelled");
            setOpen(false);
          }}
        />
      )}
    </div>
  );
}

const meta = {
  title: "Controls/ConfirmDialog",
  component: Confirm,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Confirm>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Publish: Story = {
  args: {
    title: "Publish the Deal entity?",
    message:
      "Apps in the CRM Space will use the new stage and amount fields after publication.",
    confirmLabel: "Publish",
  },
};
export const Danger: Story = {
  args: {
    title: "Remove Lena Novak from Northstar?",
    message:
      "She loses access to every Space and App in this Tenant. Her audit history is kept.",
    confirmLabel: "Remove member",
    danger: true,
  },
};
