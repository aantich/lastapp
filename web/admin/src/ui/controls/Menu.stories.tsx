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
import { Icon } from "../icon";

// menu.tsx: a WAI-ARIA menu button (↓/↑, Home/End, Escape returns focus to
// the trigger). The account menu is named by its visible text plus hidden
// words ("Ava Chen, account menu"), so the name contains what is shown.
function Account() {
  const [last, setLast] = useState("nothing yet");
  return (
    <div style={{ minHeight: 260 }} className="flex justify-end gap-6">
      <p className="text-muted">Last action: {last}</p>
      <Menu
        className="profile-menu"
        trigger={
          <>
            <Avatar name="Ava Chen" size="sm" />
            <span>Ava Chen</span>
            <span className="ui-sr-only">, account menu</span>
            <Icon icon={ChevronDown} size="sm" aria-hidden />
          </>
        }
        items={[
          {
            id: "account",
            label: "My account",
            icon: <Icon icon={UserRound} size="lg" aria-hidden />,
            onSelect: () => setLast("My account"),
          },
          {
            id: "appearance",
            label: "Appearance",
            icon: <Icon icon={Palette} size="lg" aria-hidden />,
            onSelect: () => setLast("Appearance"),
          },
          {
            id: "signout",
            label: "Sign out",
            icon: <Icon icon={LogOut} size="lg" aria-hidden />,
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
        trigger={<Icon icon={MoreHorizontal} size="xl" aria-hidden />}
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
