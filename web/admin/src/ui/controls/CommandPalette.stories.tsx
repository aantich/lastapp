import { useCallback, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Boxes, Database, Rocket, ShieldCheck, Users } from "lucide-react";
import { Button } from "../button";
import { CommandPalette, type Command } from "../command-palette";
import { useCommandPaletteShortcut } from "../hooks";
import { Kbd } from "../feedback";

// command-palette.tsx: ⌘K / Ctrl+K, filter combobox over a grouped listbox.
const icon = (I: typeof Boxes) => <I size={15} />;

function Demo({ initiallyOpen = true }: { initiallyOpen?: boolean }) {
  const [open, setOpen] = useState(initiallyOpen),
    [ran, setRan] = useState("");
  const show = useCallback(() => setOpen(true), []);
  useCommandPaletteShortcut(show);
  const commands: Command[] = [
    ["spaces", "Spaces", "Navigate", Boxes],
    ["people", "People", "Navigate", Users],
    ["access", "Access control", "Navigate", ShieldCheck],
    ["deal", "Deal", "Entities", Database, "crm sales pipeline"],
    ["invoice", "Invoice", "Entities", Database, "finance"],
    ["deploy", "Deploy Sales Desk r15", "Actions", Rocket, "release"],
    ["retire", "Retire release r12", "Actions", Rocket, "release", true],
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
        <CommandPalette commands={commands} onClose={() => setOpen(false)} />
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
