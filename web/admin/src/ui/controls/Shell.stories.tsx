import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Boxes,
  Building2,
  Database,
  Home,
  ShieldCheck,
  Users,
  Bot,
} from "lucide-react";
import { ConsoleShell } from "../../console-shell";
import type { Me } from "../../api-types";
import { ContextSwitcher } from "../context-switcher";
import { SidebarFooter, SidebarNav } from "../sidebar";
import { Button } from "../button";
import { ConsoleStory } from "../storybook/console-story";
import { policy } from "../storybook/fixtures";

// The console shell built from the kit: the ink sidebar (SidebarNav with
// aria-current and rail tooltips, SidebarFooter, the persisted collapse), the
// top bar with a context switcher, the ⌘K command palette (Navigate and
// Account commands come from the same nav and menu items) and the account
// Menu, and PageHeader with Breadcrumbs.
const me: Me = {
  id: "usr_0001",
  display_name: "Ava Chen",
  email: "ava.chen@northstar.example",
  platform_admin: false,
  auth_method: "password",
  password_change_required: false,
  tenants: [],
};
const nav = [
  { id: "overview", label: "Overview", icon: Home },
  { id: "spaces", label: "Spaces", icon: Boxes },
  { id: "people", label: "People", icon: Users },
  { id: "access", label: "Access", icon: ShieldCheck },
  { id: "agents", label: "Agents", icon: Bot },
  { id: "settings", label: "Settings", icon: Building2 },
];
const tenants = [
  { value: "ten_1", label: "Northstar", description: "7 Spaces" },
  { value: "ten_2", label: "Harbor Logistics", description: "3 Spaces" },
  { value: "ten_3", label: "Atlas Clinics", description: "5 Spaces" },
];

function Shell() {
  const [active, setActive] = useState("spaces"),
    [tenant, setTenant] = useState("ten_1");
  return (
    <ConsoleShell
      me={me}
      kind="Tenant console"
      title={nav.find((n) => n.id === active)?.label || ""}
      nav={nav}
      active={active}
      navigate={setActive}
      context={
        <ContextSwitcher
          kind="Tenant"
          value={tenant}
          options={tenants}
          onChange={setTenant}
        />
      }
      breadcrumbs={[
        { label: "Northstar", onClick: () => setActive("overview") },
        { label: nav.find((n) => n.id === active)?.label || "" },
      ]}
      commands={[
        {
          id: "space:crm",
          label: "CRM",
          group: "Spaces",
          run: () => setActive("spaces"),
        },
      ]}
      actions={<Button variant="primary">Create Space</Button>}
      appearanceControls={{
        appearance: { mode: "light" },
        setAppearance: () => {},
        reset: () => {},
      }}
      account={() => {}}
      signOut={() => {}}
    >
      <section className="panel p-6">
        <p>The {active} screen.</p>
      </section>
    </ConsoleShell>
  );
}

const meta = {
  title: "Composites/Sidebar",
  parameters: { layout: "fullscreen" },
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

// Collapse with the top-bar button; the choice survives a reload.
export const ConsoleShellStory: Story = {
  name: "Console shell",
  render: () => (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Shell />
    </ConsoleStory>
  ),
};

function Parts({ collapsed }: { collapsed: boolean }) {
  const [active, setActive] = useState("overview");
  return (
    <div className={"shell" + (collapsed ? " sidebar-collapsed" : "")}>
      <aside className="sidebar" aria-label="Navigation">
        <SidebarNav
          label="Console screens"
          section="Northstar"
          items={nav.map((n) => (n.id === "agents" ? { ...n, badge: 2 } : n))}
          active={active}
          onSelect={setActive}
          collapsed={collapsed}
        />
        <SidebarNav
          label="Spaces"
          section="Spaces"
          items={[
            { id: "crm", label: "CRM", icon: Database },
            { id: "field", label: "Field Service", icon: Database },
          ]}
          active={active}
          onSelect={setActive}
          collapsed={collapsed}
        />
        <SidebarFooter name="Ava Chen" detail="Tenant administrator" />
      </aside>
    </div>
  );
}
export const Expanded: Story = { render: () => <Parts collapsed={false} /> };
export const Rail: Story = { render: () => <Parts collapsed /> };
