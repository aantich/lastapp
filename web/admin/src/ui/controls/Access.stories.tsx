import type { Meta, StoryObj } from "@storybook/react-vite";
import { AccessMatrix, AssignmentList } from "../access-matrix";
import { Button } from "../button";
import { EmptyState } from "../empty-state";

// access-matrix.tsx: a Space's access (tenant/… access-control.tsx): one
// entity's permission paths as roles × operations, read-only, and the role
// assignments. Neutral sample data; the real screen derives every value.
const columns = ["read", "create", "update", "delete", "acl.manage"].map(
  (id) => ({ id, label: id }),
);
const granted: Record<string, string[]> = {
  reviewer: ["read", "update"],
  editor: ["read", "create", "update"],
  "rel:owner": ["read", "create", "update", "delete", "acl.manage"],
};
const draft: Record<string, string> = { "editor/delete": "added" };

function Space({ empty, readOnly }: { empty?: boolean; readOnly?: boolean }) {
  return (
    <div style={{ display: "grid", gap: 24, maxWidth: 1176 }}>
      <AccessMatrix
        title="Role matrix · Order"
        note="Published access · open a cell to edit its permission path"
        actions={<Button size="sm">Create role</Button>}
        label="Who may do what to Order"
        columns={columns}
        rows={[
          {
            id: "reviewer",
            label: "Reviewer",
            name: "Reviewer",
            detail: "Team A · where Status ≠ closed",
          },
          {
            id: "editor",
            label: "Editor",
            name: "Editor",
            detail: "No one assigned yet",
          },
          {
            id: "rel:owner",
            label: "Record owner",
            name: "Record owner",
            detail: "Relationship, not a role",
          },
        ]}
        cell={(row, op) => {
          const on = (granted[row] || []).includes(op);
          const d = draft[row + "/" + op];
          return { on, draft: d === "added" ? "added" : undefined };
        }}
        onOpen={readOnly ? undefined : () => {}}
      >
        {empty ? (
          <EmptyState compact title="No entities in this Space yet">
            Who may read and change records is set per entity.
          </EmptyState>
        ) : undefined}
      </AccessMatrix>
      <AssignmentList
        title="Assignments"
        label="Space role assignments"
        actions={
          <Button size="sm" variant="primary">
            Add access
          </Button>
        }
        items={[
          {
            id: "1",
            role: "Reviewer",
            who: "Team A",
            scope: "Space · Sample",
            action: (
              <Button size="sm" variant="danger">
                Remove
              </Button>
            ),
          },
          {
            id: "2",
            role: "Space manager",
            who: "Alex Kim",
            scope: "Space · Sample",
          },
        ]}
      />
    </div>
  );
}

const meta = { title: "Patterns/Access", component: Space } satisfies Meta<
  typeof Space
>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const ReadOnly: Story = { args: { readOnly: true } };
export const NoEntities: Story = { args: { empty: true } };
