import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Boxes,
  Building2,
  Home,
  ShieldCheck,
  Users,
  Bot,
  GitBranch,
} from "lucide-react";
import { ConsoleShell } from "../../console-shell";
import type { Me } from "../../api-types";
import { ContextSwitcher } from "../context-switcher";
import { AccountMenu, ContextLabel } from "../app-shell";
import { SidebarLinks, SidebarNav } from "../sidebar";
import { Button } from "../button";
import { ConsoleStory } from "../storybook/console-story";
import { policy } from "../storybook/fixtures";

// The console shell built from the kit: the ink sidebar (the Tenant
// switcher as section label, SidebarNav with aria-current and rail tooltips,
// the account menu, the persisted collapse), the top bar with breadcrumbs and
// search, the ⌘K command palette (Navigate and Account commands come from the
// same nav and menu items), and PageHeader.
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
      role={me.email}
      title={nav.find((n) => n.id === active)?.label || ""}
      nav={nav}
      active={active}
      navigate={setActive}
      section={
        <ContextSwitcher
          kind="Tenant"
          variant="section"
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
          section={<ContextLabel value="Example Co" />}
          items={nav.map((n) => (n.id === "agents" ? { ...n, badge: 2 } : n))}
          active={active}
          onSelect={setActive}
          collapsed={collapsed}
        />
        <SidebarLinks
          label="Spaces"
          title={<span className="ui-nav-section">Spaces</span>}
          items={[
            { id: "crm", label: "CRM", tone: "success" },
            {
              id: "field",
              label: "Field Service",
              tone: "success",
              tag: (
                <>
                  <GitBranch size={12} aria-hidden />
                  workspace
                </>
              ),
            },
          ]}
          active={active}
          onSelect={setActive}
          collapsed={collapsed}
        />
        <AccountMenu
          name="Ava Chen"
          detail="Tenant administrator"
          rail={collapsed}
          items={[]}
        />
      </aside>
    </div>
  );
}
export const Expanded: Story = { render: () => <Parts collapsed={false} /> };
export const Rail: Story = { render: () => <Parts collapsed /> };
