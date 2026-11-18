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
} from "lucide-react";
import { AccountMenu, AppShell, ContextLabel } from "../app-shell";
import { ContextSwitcher } from "../context-switcher";
import { SidebarLinks, SidebarNav } from "../sidebar";
import { Breadcrumbs, PageHeader } from "../page-header";
import { Button } from "../button";
import { Card, CardBody, CardHeader } from "../card";
import { policy } from "../storybook/fixtures";
import { ConsoleStory } from "../storybook/console-story";

// app-shell.tsx: the console frame (AppShell) as in the approved mockup: the
// 240 px ink sidebar (brand tile, collapse toggle, search, the Tenant as the
// section label or a switcher, screens with a count badge, the Spaces list
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
    (active === "crm" ? "CRM" : "crm-pricing-exp");
  return (
    <AppShell
      onSearch={() => setSearches((n) => n + 1)}
      sidebar={({ rail, close }) => (
        <>
          <SidebarNav
            label="Console screens"
            items={nav}
            active={active}
            collapsed={rail}
            section={
              switcher ? (
                <ContextSwitcher
                  kind="Tenant"
                  variant="section"
                  value="ten_1"
                  options={[
                    { value: "ten_1", label: "Northstar demo" },
                    { value: "ten_2", label: "Harbor Logistics" },
                  ]}
                  onChange={() => {}}
                />
              ) : (
                <ContextLabel value="Northstar demo" />
              )
            }
            onSelect={(id) => {
              setActive(id);
              close();
            }}
          />
          <SidebarLinks
            label="Spaces"
            collapsed={rail}
            title={<span className="ui-nav-section">Spaces</span>}
            items={[
              { id: "crm", label: "CRM", tone: "success", tag: "prod" },
              {
                id: "crm-dev",
                label: "crm-pricing-exp",
                tone: "warning",
                tag: "dev",
              },
            ]}
            active={active}
            onSelect={setActive}
          />
          <AccountMenu
            name="Maya Chen"
            detail="maya@northstar.example"
            rail={rail}
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
      )}
      breadcrumbs={
        <Breadcrumbs
          items={[
            { label: "Northstar demo", onClick: () => setActive("home") },
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

// One Tenant: its name is the sidebar's section label.
export const SingleTenant: Story = {
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Frame switcher={false} />
    </ConsoleStory>
  ),
};

// Several Tenants: the section label is a Tenant switcher.
export const TenantSwitcher: Story = {
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Frame switcher />
    </ConsoleStory>
  ),
};
