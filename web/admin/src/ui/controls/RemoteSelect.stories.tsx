import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { RemoteSelect } from "../remote-select";
import { people, policy } from "../storybook/fixtures";
import { mockApi } from "../storybook/mock-api";
import { FormField } from "../field";
import {
  ConsoleStory,
  StoryField,
  StoryFields,
} from "../storybook/console-story";

// remote-select.tsx: a searchable, paged combobox over a management list
// API. Here the list is served by mockApi from Northstar story data with the
// console's paging contract (5 per page, so "Load more" appears).
const path = "/v1/tenants/ten_northstar/memberships";

function DealOwner({ excludeInactive }: { excludeInactive: boolean }) {
  const [value, setValue] = useState("");
  return (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <StoryFields>
        <StoryField
          label="Deal owner"
          hint={value ? `user_id: ${value}` : undefined}
        >
          <RemoteSelect
            label="Deal owner"
            path={path}
            valueKey="user_id"
            value={value}
            onChange={setValue}
            initial={[]}
            required
            excludeInactive={excludeInactive}
          />
        </StoryField>
      </StoryFields>
    </ConsoleStory>
  );
}

const meta = {
  title: "Controls/RemoteSelect",
  component: DealOwner,
  args: { excludeInactive: true },
  beforeEach: () => mockApi({ [path]: people }),
} satisfies Meta<typeof DealOwner>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ActiveMembers: Story = {};
export const IncludingSuspended: Story = { args: { excludeInactive: false } };

// Outside a form grid: FormField labels it and wires its hint by id, and the
// input keeps the kit field look on its own.
function StandaloneOwner() {
  const [value, setValue] = useState("");
  return (
    <ConsoleStory passwordPolicy={policy(12, 128)}>
      <div className="max-w-md">
        <FormField
          label="Deal owner"
          hint="Only active members of Northstar can own a deal."
        >
          <RemoteSelect
            label="Deal owner"
            path={path}
            valueKey="user_id"
            value={value}
            onChange={setValue}
            initial={[]}
            excludeInactive
          />
        </FormField>
      </div>
    </ConsoleStory>
  );
}
export const InFormField: Story = { render: () => <StandaloneOwner /> };
