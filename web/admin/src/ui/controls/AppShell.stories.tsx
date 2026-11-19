import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Activity,
  AlignLeft,
  Bot,
  House,
  LayoutGrid,
  LogOut,
  Shield,
  UserRound,
  GitBranch,
} from "lucide-react";
import { AppShell } from "../app-shell";
import { AccountMenu } from "../account-menu";
import { ContextSwitcher } from "../context-switcher";
import { SidebarLinks, SidebarNav } from "../sidebar";
import { Breadcrumbs, PageHeader } from "../page-header";
import { Button } from "../button";
import { Card, CardBody, CardHeader } from "../card";
import { policy } from "../storybook/fixtures";
import { ConsoleStory } from "../storybook/console-story";
import { Icon } from "../icon";

// app-shell.tsx: the console frame (AppShell) as in the approved mockup: the
// 240 px ink sidebar (brand tile and collapse toggle, the Tenant switcher
// under them, search, screens with a count badge, the Spaces list
// and the account menu at the bottom), a rail at 68 px, a drawer under
// 800 px; the top bar with breadcrumbs, search and the primary action; the
// content column centred at 1240 px.
const meta = {
  title: "Composites/App shell",
  parameters: { layout: "fullscreen" },
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const nav = [
  { id: "home", label: "Home", icon: House },
  { id: "apps", label: "Apps", icon: LayoutGrid },
  { id: "entities", label: "Entities", icon: AlignLeft },
  { id: "access", label: "Access", icon: Shield },
  { id: "agents", label: "Agents", icon: Bot, badge: 2 },
  { id: "activity", label: "Activity", icon: Activity },
];

function Frame({ switcher }: { switcher: boolean }) {
  const [active, setActive] = useState("home"),
    [searches, setSearches] = useState(0);
  const label =
    nav.find((n) => n.id === active)?.label ||
    (active === "crm" ? "CRM" : "sales-exp");
  return (
    <AppShell
      onSearch={() => setSearches((n) => n + 1)}
      context={(rail) => (
        <ContextSwitcher
          kind="Tenant"
          variant="sidebar"
          rail={rail}
          value="ten_1"
          options={[
            { value: "ten_1", label: "Example Co" },
            ...(switcher
              ? [{ value: "ten_2", label: "Harbor Logistics" }]
              : []),
          ]}
          onChange={() => {}}
        />
      )}
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
          <SidebarLinks
            label="Spaces"
            collapsed={rail}
            title="Spaces"
            items={[
              { id: "crm", label: "CRM", tone: "success" },
              {
                id: "crm-dev",
                label: "sales-exp",
                tone: "success",
                tag: (
                  <>
                    <Icon icon={GitBranch} size="2xs" aria-hidden />
                    workspace
                  </>
                ),
              },
            ]}
            active={active}
            onSelect={setActive}
          />
          <AccountMenu
            name="Alex Kim"
            detail="alex@example.test"
            rail={rail}
            items={[
              {
                id: "account",
                label: "My account",
                icon: <Icon icon={UserRound} size="lg" aria-hidden />,
                onSelect: () => {},
              },
              {
                id: "signout",
                label: "Sign out",
                icon: <Icon icon={LogOut} size="lg" aria-hidden />,
                onSelect: () => {},
              },
            ]}
          />
        </>
      )}
      breadcrumbs={
        <Breadcrumbs
          items={[
            { label: "Example Co", onClick: () => setActive("home") },
            { label },
          ]}
        />
      }
      actions={<Button variant="primary">Connect agent</Button>}
    >
      <PageHeader title={label} />
      <Card>
        <CardHeader title={`The ${label} screen`} />
        <CardBody>Search opened {searches} times.</CardBody>
      </Card>
    </AppShell>
  );
}

// One Tenant: the Tenant switcher still shows under the brand row, so switching
// stays discoverable; it lists the one Tenant.
export const SingleTenant: Story = {
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Frame switcher={false} />
    </ConsoleStory>
  ),
};

// Several Tenants: the switcher lists them all.
export const TenantSwitcher: Story = {
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Frame switcher />
    </ConsoleStory>
  ),
};
