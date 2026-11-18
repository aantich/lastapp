import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Boxes,
  Home,
  LogOut,
  ShieldCheck,
  UserRound,
  Users,
} from "lucide-react";
import { AccountMenu, AppShell, ContextLabel, QuickSearch } from "../app-shell";
import { ContextSwitcher } from "../context-switcher";
import { SidebarFooter, SidebarNav } from "../sidebar";
import { PageHeader } from "../page-header";
import { Card, CardBody, CardHeader } from "../card";
import { policy, spaces } from "../storybook/fixtures";
import { ConsoleStory } from "../storybook/console-story";

// app-shell.tsx: the console frame (AppShell: sidebar that collapses to a
// rail or becomes a drawer under 800 px, top bar, content and footer), the
// top bar's QuickSearch trigger, AccountMenu and ContextLabel (a fixed
// Tenant). The Space switcher passed as `secondaryContext` hides on narrow
// screens.
const meta = {
  title: "Composites/App shell",
  parameters: { layout: "fullscreen" },
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const nav = [
  { id: "home", label: "Home", icon: Home },
  { id: "spaces", label: "Spaces", icon: Boxes },
  { id: "people", label: "People", icon: Users },
  { id: "access", label: "Access", icon: ShieldCheck },
];

function Frame({ switcher }: { switcher: boolean }) {
  const [active, setActive] = useState("home"),
    [space, setSpace] = useState<string | undefined>(undefined),
    [searches, setSearches] = useState(0);
  const label = nav.find((n) => n.id === active)?.label || "";
  return (
    <AppShell
      kind="Tenant"
      sidebar={({ rail, close }) => (
        <>
          <SidebarNav
            label="Console screens"
            items={nav}
            active={active}
            collapsed={rail}
            onSelect={(id) => {
              setActive(id);
              close();
            }}
          />
          <SidebarFooter name="Ava Chen" detail="ava.chen@northstar.example" />
        </>
      )}
      context={
        switcher ? (
          <ContextSwitcher
            kind="Tenant"
            value="ten_1"
            options={[
              { value: "ten_1", label: "Northstar" },
              { value: "ten_2", label: "Harbor Logistics" },
            ]}
            onChange={() => {}}
          />
        ) : (
          <ContextLabel kind="Tenant" value="Northstar" />
        )
      }
      secondaryContext={
        <ContextSwitcher
          kind="Space"
          value={space}
          placeholder="All Spaces"
          options={spaces.map((s) => ({
            value: String(s.id),
            label: String(s.display_name),
          }))}
          onChange={setSpace}
        />
      }
      actions={
        <>
          <QuickSearch onOpen={() => setSearches((n) => n + 1)} />
          <AccountMenu
            name="Ava Chen"
            items={[
              {
                id: "account",
                label: "My account",
                icon: <UserRound size={16} aria-hidden />,
                onSelect: () => {},
              },
              {
                id: "signout",
                label: "Sign out",
                icon: <LogOut size={16} aria-hidden />,
                onSelect: () => {},
              },
            ]}
          />
        </>
      }
      footer={<span>© 2026 Lastapp</span>}
    >
      <PageHeader title={label} />
      <Card>
        <CardHeader title={`The ${label} screen`} />
        <CardBody>Search opened {searches} times.</CardBody>
      </Card>
    </AppShell>
  );
}

// One Tenant: the name is a fixed ContextLabel.
export const SingleTenant: Story = {
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Frame switcher={false} />
    </ConsoleStory>
  ),
};

// Several Tenants: the context is a ContextSwitcher.
export const TenantSwitcher: Story = {
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Frame switcher />
    </ConsoleStory>
  ),
};
