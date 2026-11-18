import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { DatePicker } from "../date-picker";
import { addDays, isoDay } from "../days";
import { FormField } from "../field";
import { StoryFields } from "../storybook/console-story";

// date-picker.tsx: a button that opens a calendar dialog (no native date
// input). Arrows move a day/week, PageUp/PageDown a month, Enter chooses,
// Escape closes; days outside min/max are disabled.
function Expiry({
  maxDays,
  initialDays,
}: {
  maxDays: number;
  initialDays?: number;
}) {
  const today = new Date();
  const [value, setValue] = useState(
    initialDays === undefined ? "" : isoDay(addDays(today, initialDays)),
  );
  return (
    <StoryFields>
      <FormField
        label="Access expires on"
        hint={`At most ${maxDays} days from today.`}
      >
        <DatePicker
          value={value}
          onChange={setValue}
          min={isoDay(addDays(today, 1))}
          max={isoDay(addDays(today, maxDays))}
        />
      </FormField>
    </StoryFields>
  );
}

const meta = {
  title: "Controls/DatePicker",
  component: Expiry,
} satisfies Meta<typeof Expiry>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Chosen: Story = { args: { maxDays: 90, initialDays: 30 } };
export const Empty: Story = { args: { maxDays: 365 } };
