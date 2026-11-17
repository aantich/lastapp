import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Checkbox, RadioGroup, Switch } from "../checkbox";

// checkbox.tsx: Checkbox (native, custom face, indeterminate), Switch
// (role=switch) and RadioGroup (fieldset of native radios).
const meta = { title: "Primitives/Checkbox" } satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

function Checkboxes() {
  const [values, setValues] = useState({
    read: true,
    create: false,
    update: true,
  });
  const all = Object.values(values);
  const set = (key: keyof typeof values) => (on: boolean) =>
    setValues({ ...values, [key]: on });
  return (
    <div className="grid gap-3">
      <Checkbox
        label="All Deal permissions"
        checked={all.every(Boolean)}
        indeterminate={all.some(Boolean) && !all.every(Boolean)}
        onChange={(on) => setValues({ read: on, create: on, update: on })}
      />
      <div className="grid gap-3 pl-7">
        <Checkbox label="Read" checked={values.read} onChange={set("read")} />
        <Checkbox
          label="Create"
          checked={values.create}
          onChange={set("create")}
        />
        <Checkbox
          label="Update"
          description="Only fields in the role's allowlist."
          checked={values.update}
          onChange={set("update")}
        />
      </div>
      <Checkbox
        label="Delete (locked by the Space)"
        checked={false}
        disabled
        onChange={() => {}}
      />
      <Checkbox
        label="Audit (always on)"
        checked
        disabled
        onChange={() => {}}
      />
    </div>
  );
}
export const Checkboxes_: Story = {
  name: "Checkbox",
  render: () => <Checkboxes />,
};

function Switches() {
  const [on, setOn] = useState(true),
    [off, setOff] = useState(false);
  return (
    <div className="grid gap-4">
      <Switch
        label="Allow agent access"
        description="Agents act only within the delegating person's authority."
        checked={on}
        onChange={setOn}
      />
      <Switch label="Email digest" checked={off} onChange={setOff} />
      <Switch
        label="Single sign-on (managed by the platform)"
        checked
        disabled
        onChange={() => {}}
      />
    </div>
  );
}
export const Switch_: Story = { name: "Switch", render: () => <Switches /> };

function Radios({ horizontal }: { horizontal?: boolean }) {
  const [value, setValue] = useState("system");
  return (
    <RadioGroup
      legend="Color mode"
      horizontal={horizontal}
      value={value}
      onChange={setValue}
      options={[
        { value: "light", label: "Light" },
        { value: "dark", label: "Dark" },
        {
          value: "system",
          label: "System",
          description: "Follows the operating system.",
        },
        { value: "contrast", label: "High contrast", disabled: true },
      ]}
    />
  );
}
export const Radio: Story = { render: () => <Radios /> };
export const RadioHorizontal: Story = { render: () => <Radios horizontal /> };
