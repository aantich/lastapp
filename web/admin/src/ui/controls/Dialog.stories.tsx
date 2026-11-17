import { useState, type ComponentProps } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../button";
import { Dialog, DialogBody } from "../dialog";
import { TextField } from "../field";

// dialog.tsx: the one modal shell (focus trap, Escape, aria-modal, labelled
// by its title). FormModal and ConfirmDialog are wrappers on it.
function Demo(
  props: Partial<ComponentProps<typeof Dialog>> & { busy?: boolean },
) {
  const [open, setOpen] = useState(true);
  return (
    <div className="p-4">
      <Button onClick={() => setOpen(true)}>Open dialog</Button>
      {open && (
        <Dialog
          title="Rename Space"
          description="The slug stays the same, so App URLs keep working."
          onClose={() => setOpen(false)}
          dismissable={!props.busy}
          footer={
            <>
              <Button
                variant="ghost"
                disabled={props.busy}
                onClick={() => setOpen(false)}
              >
                Cancel
              </Button>
              <Button variant="primary" loading={props.busy}>
                {props.busy ? "Saving…" : "Save"}
              </Button>
            </>
          }
          {...props}
        >
          <DialogBody>
            <TextField label="Name" defaultValue="Field Service" required />
          </DialogBody>
        </Dialog>
      )}
    </div>
  );
}

const meta = {
  title: "Primitives/Dialog",
  component: Demo,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Demo>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Busy: Story = { args: { busy: true } };
export const Small: Story = {
  args: {
    size: "sm",
    role: "alertdialog",
    title: "Discard draft changes?",
    description: "The 3 unpublished changes to Invoice are lost.",
    children: null,
  },
};
export const Large: Story = {
  args: {
    size: "lg",
    title: "Agent access",
    description: "Claude Desktop · connected by Ava Chen",
  },
};
