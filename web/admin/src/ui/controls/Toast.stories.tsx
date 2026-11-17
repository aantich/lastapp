import { useEffect } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../button";
import { Toaster } from "../toast";
import { dismissToast, toast } from "../toast-store";

// toast.tsx / toast-store.ts: short confirmations in aria-live regions
// (polite; danger assertive). Toaster is mounted once in main.tsx.
function Demo({ preload }: { preload?: boolean }) {
  useEffect(() => {
    if (!preload) return;
    const ids = [
      toast("Sales manager saved as draft", { tone: "success", duration: 0 }),
      toast("Release r15 queued", {
        duration: 0,
        action: { label: "Open", onClick: () => {} },
      }),
      toast("Deploy r14 failed", { tone: "danger" }),
    ];
    return () => ids.forEach(dismissToast);
  }, [preload]);
  return (
    <div className="flex flex-wrap gap-3 p-4">
      <Button
        onClick={() => toast("Ava Chen added to CRM", { tone: "success" })}
      >
        Success toast
      </Button>
      <Button
        onClick={() =>
          toast("Field archived", {
            action: { label: "Undo", onClick: () => toast("Field restored") },
          })
        }
      >
        With action
      </Button>
      <Button
        variant="danger"
        onClick={() => toast("Could not reach db-eu-1", { tone: "danger" })}
      >
        Danger toast
      </Button>
      <Toaster />
    </div>
  );
}

const meta = {
  title: "Primitives/Toast",
  component: Demo,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Demo>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Interactive: Story = {};
export const Stack: Story = { args: { preload: true } };
