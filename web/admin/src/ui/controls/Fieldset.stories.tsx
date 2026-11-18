import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Fieldset } from "../fieldset";
import { Checkbox } from "../checkbox";
import { FormField } from "../field";
import { NumberInput } from "../number-input";
import { Stack } from "../layout";

// fieldset.tsx: a titled group of controls inside one form. The legend is
// announced when focus enters the group; the description says what the
// group decides.
const permissions = [
  ["read", "Read configuration", "Spaces, Apps, entities and releases."],
  [
    "entities.draft",
    "Draft entities",
    "Change entity drafts; nothing is live.",
  ],
  ["entities.publish", "Publish entities", "Make a checked draft live."],
] as const;

function AgentPermissions() {
  const [granted, setGranted] = useState<string[]>(["read"]);
  const [databases, setDatabases] = useState<number | "">(2);
  return (
    <form className="max-w-2xl">
      <Stack gap="md">
        <Fieldset legend="What may this agent do?">
          {permissions.map(([code, label, description]) => (
            <Checkbox
              key={code}
              label={label}
              description={description}
              checked={granted.includes(code)}
              onChange={(on) =>
                setGranted(
                  on ? [...granted, code] : granted.filter((c) => c !== code),
                )
              }
            />
          ))}
        </Fieldset>
        <Fieldset
          legend="Provisioning limits"
          description="Only approved targets may receive new Northstar databases."
        >
          <FormField label="Maximum new databases">
            <NumberInput
              min={1}
              max={5}
              steppers
              value={databases}
              onChange={setDatabases}
            />
          </FormField>
        </Fieldset>
      </Stack>
    </form>
  );
}

const meta = {
  title: "Primitives/Fieldset",
  component: AgentPermissions,
} satisfies Meta<typeof AgentPermissions>;
export default meta;
type Story = StoryObj<typeof meta>;

export const GroupedControls: Story = {};
