import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { RoleMatrix } from "../role-matrix";

// role-matrix.tsx: roles × permissions with column "mixed" headers.
const rows = [
  { id: "sales_rep", name: "Sales rep", description: "Own Deals only" },
  { id: "sales_manager", name: "Sales manager", description: "Team's Deals" },
  { id: "finance", name: "Finance reviewer", description: "Closed Deals" },
  { id: "auditor", name: "Auditor" },
];
const columns = ["read", "create", "update", "delete", "acl"].map((id) => ({
  id,
  label: id[0].toUpperCase() + id.slice(1),
}));

function Demo({ disabled }: { disabled?: boolean }) {
  const [value, setValue] = useState<Record<string, string[]>>({
    sales_rep: ["read", "create", "update"],
    sales_manager: ["read", "create", "update", "delete"],
    finance: ["read"],
    auditor: ["read"],
  });
  return (
    <section className="panel">
      <RoleMatrix
        label="Deal permissions by role"
        rows={rows}
        columns={columns}
        value={value}
        onChange={setValue}
        disabled={disabled}
      />
    </section>
  );
}

const meta = { title: "Composites/RoleMatrix", component: Demo } satisfies Meta<
  typeof Demo
>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Editable: Story = {};
export const ReadOnly: Story = { args: { disabled: true } };
