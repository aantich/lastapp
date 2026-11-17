import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Select, type ListOption } from "../select";
import { dealStages } from "../storybook/fixtures";
import { StoryField, StoryFields } from "../storybook/console-story";

// select.tsx: the console's only choice control (native <select> is not
// allowed). Keyboard: arrows, Home/End, type-ahead, Enter/Escape.
function DealStage(props: {
  options: ListOption[];
  initial?: string;
  disabled?: boolean;
  required?: boolean;
}) {
  const [value, setValue] = useState(props.initial ?? "");
  return (
    <StoryFields>
      <StoryField
        label="Deal stage"
        hint={value ? `Value: ${value}` : undefined}
      >
        <Select
          aria-label="Deal stage"
          options={props.options}
          value={value}
          onChange={setValue}
          disabled={props.disabled}
          required={props.required}
        />
      </StoryField>
    </StoryFields>
  );
}

const meta = {
  title: "Controls/Select",
  component: DealStage,
  args: { options: dealStages, required: true },
} satisfies Meta<typeof DealStage>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {};
export const Chosen: Story = { args: { initial: "proposal" } };
export const Disabled: Story = { args: { initial: "won", disabled: true } };
