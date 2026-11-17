import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  ChevronDown,
  LogOut,
  MoreHorizontal,
  Palette,
  UserRound,
} from "lucide-react";
import { Menu } from "../menu";
import { Avatar } from "../avatar";

// menu.tsx: a WAI-ARIA menu button (↓/↑, Home/End, Escape returns focus to
// the trigger). The account menu's name contains its visible text.
function Account() {
  const [last, setLast] = useState("nothing yet");
  return (
    <div style={{ minHeight: 260 }} className="flex justify-end gap-6">
      <p className="text-muted">Last action: {last}</p>
      <Menu
        className="profile-menu"
        label="Ava Chen, account menu"
        trigger={
          <>
            <Avatar name="Ava Chen" size={28} />
            <span>Ava Chen</span>
            <ChevronDown size={14} aria-hidden />
          </>
        }
        items={[
          {
            id: "account",
            label: "My account",
            icon: <UserRound size={16} aria-hidden />,
            onSelect: () => setLast("My account"),
          },
          {
            id: "appearance",
            label: "Appearance",
            icon: <Palette size={16} aria-hidden />,
            onSelect: () => setLast("Appearance"),
          },
          {
            id: "signout",
            label: "Sign out",
            icon: <LogOut size={16} aria-hidden />,
            onSelect: () => setLast("Sign out"),
          },
        ]}
      />
    </div>
  );
}

function RowActions() {
  const [last, setLast] = useState("nothing yet");
  return (
    <div style={{ minHeight: 220 }} className="flex gap-6">
      <Menu
        align="start"
        label="Deal review actions"
        trigger={<MoreHorizontal size={18} aria-hidden />}
        items={[
          { id: "open", label: "Open App", onSelect: () => setLast("Open") },
          {
            id: "rename",
            label: "Rename",
            onSelect: () => setLast("Rename"),
          },
          {
            id: "archive",
            label: "Archive",
            tone: "danger",
            onSelect: () => setLast("Archive"),
          },
        ]}
      />
      <p className="text-muted">Last action: {last}</p>
    </div>
  );
}

const meta = { title: "Composites/Menu" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const AccountMenu: Story = { render: () => <Account /> };
export const IconTrigger: Story = { render: () => <RowActions /> };
