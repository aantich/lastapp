import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { NumberInput } from "../../number-input";
import { StoryField, StoryFields } from "../storybook/console-story";

// number-input.tsx: a text field for numbers with no native stepper. Values
// outside min/max clamp on blur; ArrowUp/ArrowDown step.
function Amount(props: {
  label: string;
  initial: number | "";
  min?: number;
  max?: number;
  step?: number;
  decimal?: boolean;
  steppers?: boolean;
  hint?: string;
}) {
  const [value, setValue] = useState<number | "">(props.initial);
  return (
    <StoryFields>
      <StoryField label={props.label} hint={props.hint}>
        <NumberInput
          aria-label={props.label}
          value={value}
          onChange={setValue}
          min={props.min}
          max={props.max}
          step={props.step}
          decimal={props.decimal}
          steppers={props.steppers}
        />
      </StoryField>
    </StoryFields>
  );
}

const meta = {
  title: "Controls/NumberInput",
  component: Amount,
} satisfies Meta<typeof Amount>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DealAmount: Story = {
  args: {
    label: "Deal amount (EUR)",
    initial: 48500,
    min: 0,
    decimal: true,
    hint: "Northstar CRM · Renewal — Acme Logistics",
  },
};
export const WithSteppers: Story = {
  args: {
    label: "Seats",
    initial: 25,
    min: 1,
    max: 500,
    steppers: true,
    hint: "Clamped to 1–500 on blur.",
  },
};
