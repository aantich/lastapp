import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { FormField, TextField, Textarea } from "../field";
import { Select } from "../select";
import { dealStages } from "../storybook/fixtures";

// field.tsx: label/hint/error wiring by id for any control.
const meta = { title: "Primitives/TextField" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const States: Story = {
  render: () => (
    <div className="grid max-w-md gap-5">
      <TextField label="Space name" placeholder="e.g. Field Service" required />
      <TextField
        label="Slug"
        defaultValue="field-service"
        hint="Used in App URLs; cannot change later."
      />
      <TextField
        label="Email"
        type="email"
        defaultValue="ava.chen@"
        error="Enter a full email address, e.g. ava.chen@northstar.example."
      />
      <TextField
        label="Database host"
        defaultValue="db-eu-1.internal"
        disabled
      />
      <Textarea
        label="Description"
        defaultValue="Work orders and technician schedules"
        hint="Shown to Space members."
      />
    </div>
  ),
};

function WithSelect() {
  const [stage, setStage] = useState("");
  return (
    <div className="max-w-md">
      <FormField
        label="Deal stage"
        required
        error={stage ? undefined : "Choose a stage."}
      >
        <Select options={dealStages} value={stage} onChange={setStage} />
      </FormField>
    </div>
  );
}
export const AnyControl: Story = { render: () => <WithSelect /> };

// A row that already says what the control is (an expression's value):
// the label stays for assistive technology only.
export const HiddenLabel: Story = {
  render: () => (
    <div className="max-w-md">
      <TextField hideLabel label="Condition value" defaultValue="qualified" />
    </div>
  ),
};
