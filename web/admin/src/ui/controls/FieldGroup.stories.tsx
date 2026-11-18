import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Card, CardBody, CardHeader } from "../card";
import { Checkbox } from "../checkbox";
import { Columns, Stack } from "../layout";
import { Select } from "../select";
import { TextField } from "../field";
import { Button } from "../button";
import { FieldGroup } from "../field-group";
import { dealStages } from "../storybook/fixtures";

// field-group.tsx: related controls under one legend (a native fieldset).
// Framed groups are repeatable units; nested groups indent a sub-part.
const meta = { title: "Form controls/FieldGroup" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

const dealFields = ["Name", "Amount", "Stage", "Company"];

function PermissionPath() {
  const [fields, setFields] = useState(["Name", "Amount"]);
  const [stage, setStage] = useState("qualified");
  return (
    <Card>
      <CardHeader title="Sales reviewer can read" level={3} />
      <CardBody>
        <Stack gap="md">
          <FieldGroup
            legend="Which fields"
            description="New fields never join this list automatically."
          >
            <Columns min={140} gap="sm">
              {dealFields.map((f) => (
                <Checkbox
                  key={f}
                  label={f}
                  checked={fields.includes(f)}
                  onChange={(on) =>
                    setFields(
                      on ? [...fields, f] : fields.filter((x) => x !== f),
                    )
                  }
                />
              ))}
            </Columns>
          </FieldGroup>
          <FieldGroup legend="Which records" nested>
            <Columns min={180} gap="sm">
              <Select
                aria-label="Condition field"
                value="stage"
                onChange={() => {}}
                options={[{ value: "stage", label: "Stage" }]}
              />
              <Select
                aria-label="Condition value"
                value={stage}
                onChange={setStage}
                options={dealStages}
              />
            </Columns>
          </FieldGroup>
        </Stack>
      </CardBody>
    </Card>
  );
}

export const PathGroups: Story = { render: () => <PermissionPath /> };

export const FramedAndDisabled: Story = {
  render: () => (
    <Stack gap="md">
      <FieldGroup
        legend="Permission gate 1"
        framed
        actions={
          <Button variant="link" className="danger">
            Remove gate
          </Button>
        }
      >
        <TextField
          label="App origin"
          defaultValue="https://crm.northstar.example"
        />
      </FieldGroup>
      <FieldGroup
        legend="Space approval"
        description="A Space manager approves this origin and ceiling."
        framed
        disabled
      >
        <TextField
          label="App origin"
          defaultValue="https://crm.northstar.example"
        />
      </FieldGroup>
    </Stack>
  ),
};
