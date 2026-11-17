import type { Meta, StoryObj } from "@storybook/react-vite";
import { Collection } from "../collection";
import { text } from "../../api-types";
import { nameColumn, statusColumn } from "../../tenant/screen";
import { policy, spaces } from "../storybook/fixtures";
import { mockApi } from "../storybook/mock-api";
import { ConsoleStory } from "../storybook/console-story";

// collection.tsx: every Tenant list screen. Search and paging run on the
// server; mockApi serves Northstar's Spaces with the same contract
// (5 per page). Columns are the console's own name/status columns.
const path = "/v1/tenants/ten_northstar/spaces";

function Spaces({ path, empty }: { path: string; empty?: string }) {
  return (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <Collection
        path={path}
        columns={[
          nameColumn,
          ["Description", (r) => text(r.description)],
          ["Apps", (r) => text(r.app_count)],
          statusColumn,
        ]}
        empty={empty}
        actions={() => <button className="text-button">Open</button>}
      />
    </ConsoleStory>
  );
}

const meta = {
  title: "Controls/Collection",
  component: Spaces,
  beforeEach: () =>
    mockApi({ [path]: spaces, "/v1/tenants/ten_empty/spaces": [] }),
} satisfies Meta<typeof Spaces>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NorthstarSpaces: Story = { args: { path } };
export const Empty: Story = {
  args: {
    path: "/v1/tenants/ten_empty/spaces",
    empty: "Create a Space to organize your Apps.",
  },
};
export const LoadError: Story = {
  args: { path: "/v1/tenants/ten_missing/spaces" },
};
