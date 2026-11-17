import type { Meta, StoryObj } from "@storybook/react-vite";
import { Pencil } from "lucide-react";
import { DataTable, type Column } from "../data-table";
import { StatusBadge } from "../status";
import { IconButton } from "../button";
import { spaces } from "../storybook/fixtures";
import { text, type Item } from "../../api-types";

// data-table.tsx: the console table; Collection adds server search/paging.
const columns: Column<Item>[] = [
  {
    key: "name",
    header: "Space",
    render: (r) => <strong>{text(r.display_name)}</strong>,
  },
  { key: "slug", header: "Slug", render: (r) => <code>{text(r.slug)}</code> },
  {
    key: "apps",
    header: "Apps",
    numeric: true,
    render: (r) => text(r.app_count),
  },
  {
    key: "state",
    header: "State",
    render: (r) => <StatusBadge state={r.state} />,
  },
];

function Demo({ loading, empty }: { loading?: boolean; empty?: boolean }) {
  return (
    <section className="panel list-panel">
      <DataTable
        caption="Spaces in Northstar"
        columns={columns}
        rows={empty ? [] : spaces}
        rowKey={(r) => text(r.id)}
        loading={loading}
        empty="No Spaces yet. Create one to bind a database."
        actions={(r) => (
          <IconButton
            label={`Edit ${text(r.display_name)}`}
            icon={<Pencil size={15} aria-hidden />}
          />
        )}
      />
    </section>
  );
}

const meta = { title: "Composites/DataTable", component: Demo } satisfies Meta<
  typeof Demo
>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Rows: Story = {};
export const Loading: Story = { args: { loading: true } };
export const Empty: Story = { args: { empty: true } };
