import type { Meta, StoryObj } from "@storybook/react-vite";
import { Plus, Rocket } from "lucide-react";
import { Button } from "../button";
import { Breadcrumbs, PageHeader } from "../page-header";

// page-header.tsx: the page's h1 block and the Tenant → Space → App trail.
const meta = { title: "Primitives/PageHeader" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const Full: Story = {
  render: () => (
    <PageHeader
      breadcrumbs={
        <Breadcrumbs
          items={[
            { label: "Northstar", onClick: () => {} },
            { label: "CRM", onClick: () => {} },
            { label: "Sales Desk" },
          ]}
        />
      }
      title="Sales Desk"
      meta="app_crm_0001 · sales.northstar.example"
      description="The account executives' pipeline App. It reuses the CRM Space's Deal and Account entities."
      actions={
        <>
          <Button>Settings</Button>
          <Button variant="primary" icon={<Rocket size={16} aria-hidden />}>
            Deploy
          </Button>
        </>
      }
    />
  ),
};

export const Minimal: Story = {
  render: () => (
    <PageHeader
      title="Spaces"
      actions={
        <Button variant="primary" icon={<Plus size={16} aria-hidden />}>
          Create Space
        </Button>
      }
    />
  ),
};

export const BreadcrumbTrail: Story = {
  render: () => (
    <Breadcrumbs
      items={[
        { label: "Northstar", href: "#tenant" },
        { label: "Field Service", href: "#space" },
        { label: "Work orders", href: "#app" },
        { label: "Release r15" },
      ]}
    />
  ),
};
