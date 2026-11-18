import { useCallback, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Boxes, Database, Rocket, ShieldCheck, Users } from "lucide-react";
import { Button } from "../button";
import { CommandPalette, type Command } from "../command-palette";
import { useCommandPaletteShortcut } from "../hooks";
import { Kbd } from "../feedback";

// command-palette.tsx: ⌘K / Ctrl+K, filter combobox over a listbox whose
// rows end with their kind.
const icon = (I: typeof Boxes) => <I size={15} />;

function Demo({ initiallyOpen = true }: { initiallyOpen?: boolean }) {
  const [open, setOpen] = useState(initiallyOpen),
    [ran, setRan] = useState("");
  const show = useCallback(() => setOpen(true), []);
  useCommandPaletteShortcut(show);
  const commands: Command[] = [
    ["spaces", "Go to Spaces", "Navigate", Boxes],
    ["people", "Go to People", "Navigate", Users],
    ["access", "Go to Access", "Navigate", ShieldCheck],
    ["order", "Open Order entity", "Entities", Database, "order"],
    ["invoice", "Open Invoice entity", "Entities", Database, "invoice"],
    ["deploy", "Retry Sales Desk deploy", "Action", Rocket, "release"],
    ["retire", "Retire release r12", "Action", Rocket, "release", true],
  ].map(([id, label, group, I, keywords, disabled]) => ({
    id: id as string,
    label: label as string,
    group: group as string,
    icon: icon(I as typeof Boxes),
    keywords: keywords as string | undefined,
    disabled: disabled as boolean | undefined,
    run: () => setRan(label as string),
  }));
  return (
    <div className="grid justify-items-start gap-3 p-4">
      <Button onClick={show}>
        Search or run… <Kbd>⌘K</Kbd>
      </Button>
      {ran && <p role="status">Ran: {ran}</p>}
      {open && (
        <CommandPalette
          commands={commands}
          onClose={() => setOpen(false)}
          examples={["order", "deploy"]}
        />
      )}
    </div>
  );
}

const meta = {
  title: "Composites/CommandPalette",
  component: Demo,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Demo>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {};
export const Closed: Story = { args: { initiallyOpen: false } };
