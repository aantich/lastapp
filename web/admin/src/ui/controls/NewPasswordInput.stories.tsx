import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { NewPasswordInput } from "../../new-password-input";
import { policy } from "../storybook/fixtures";
import {
  ConsoleStory,
  StoryField,
  StoryFields,
} from "../storybook/console-story";

// new-password-input.tsx enforces the server's password policy (counted in
// Unicode code points). The policy is a story arg, as GET /v1/config
// supplies it to the console: change min/max to see validation follow it.
function NewPassword({ min, max }: { min: number; max: number }) {
  const [value, setValue] = useState(""),
    [saved, setSaved] = useState(false);
  const supplied = policy(min, max);
  return (
    <ConsoleStory passwordPolicy={supplied}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setSaved(true);
        }}
      >
        <StoryFields>
          <StoryField
            label="New password for Ava Chen"
            hint={`Use a password of ${supplied.length_hint}`}
          >
            <NewPasswordInput
              aria-label="New password"
              type="password"
              autoComplete="new-password"
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                setSaved(false);
              }}
            />
          </StoryField>
        </StoryFields>
        <p className="mt-4 flex items-center gap-3">
          <button className="primary">Set password</button>
          {saved && <span role="status">Accepted by the policy.</span>}
        </p>
      </form>
    </ConsoleStory>
  );
}

const meta = {
  title: "Controls/NewPasswordInput",
  component: NewPassword,
  argTypes: {
    min: { control: { type: "range", min: 1, max: 64 } },
    max: { control: { type: "range", min: 8, max: 256 } },
  },
} satisfies Meta<typeof NewPassword>;
export default meta;
type Story = StoryObj<typeof meta>;

export const SuppliedPolicy: Story = { args: { min: 12, max: 128 } };
export const StricterPolicy: Story = { args: { min: 20, max: 64 } };
