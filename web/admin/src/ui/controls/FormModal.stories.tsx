import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { FormModal, type Modal } from "../../form-modal";
import { people, policy } from "../storybook/fixtures";
import { mockApi } from "../storybook/mock-api";
import { ConsoleStory } from "../storybook/console-story";

// form-modal.tsx: the shared create/edit dialog. Fields reuse Select,
// RemoteSelect, NumberInput and NewPasswordInput; Save runs through
// useAction. Here save() resolves after a short delay (or fails, to show the
// error state) instead of calling the API.
const members = "/v1/tenants/ten_northstar/memberships";
const wait = () => new Promise((resolve) => setTimeout(resolve, 600));

const createSpace: Omit<Modal, "save"> = {
  title: "Create Space",
  description:
    "A Space in Northstar owns one database and the entities its Apps reuse.",
  initial: { display_name: "", slug: "" },
  fields: [
    {
      key: "display_name",
      label: "Name",
      required: true,
      hint: "For example CRM or Field Service.",
    },
    { key: "slug", label: "Slug", required: true, advanced: true },
    { key: "description", label: "Description", wide: true },
    {
      key: "manager_user_id",
      label: "Space manager",
      type: "select",
      lookup: members,
    },
    {
      key: "database_mode",
      label: "Database",
      type: "select",
      options: [
        { value: "shared", label: "Shared Northstar server" },
        { value: "dedicated", label: "Dedicated database" },
      ],
    },
    { key: "max_apps", label: "App limit", type: "number", advanced: true },
  ],
};

const createAccount: Omit<Modal, "save"> = {
  title: "Create account",
  description:
    "Adds a local account to Northstar. They change the password at first sign-in.",
  initial: {},
  fields: [
    { key: "display_name", label: "Full name", required: true },
    { key: "email", label: "Email", type: "email", required: true },
    {
      key: "password",
      label: "Initial password",
      type: "password",
      newPassword: true,
      generate: true,
      required: true,
    },
    {
      key: "platform_admin",
      label: "Platform administrator",
      type: "checkbox",
    },
  ],
};

function Dialog({
  form,
  fail,
  min,
  max,
}: {
  form: Omit<Modal, "save">;
  fail: boolean;
  min: number;
  max: number;
}) {
  const [open, setOpen] = useState(true),
    [result, setResult] = useState("");
  return (
    <ConsoleStory passwordPolicy={policy(min, max)}>
      <div className="grid justify-items-start gap-3 p-4">
        <button className="primary" onClick={() => setOpen(true)}>
          {form.title}
        </button>
        {result && <p role="status">{result}</p>}
      </div>
      {open && (
        <FormModal
          modal={{
            ...form,
            save: async () => {
              await wait();
              if (fail)
                throw new Error(
                  "A Space with this slug already exists in Northstar.",
                );
            },
          }}
          lookups={{}}
          close={() => setOpen(false)}
          done={() => {
            setOpen(false);
            setResult("Saved.");
          }}
        />
      )}
    </ConsoleStory>
  );
}

const meta = {
  title: "Controls/FormModal",
  component: Dialog,
  args: { fail: false, min: 12, max: 128 },
  parameters: { layout: "fullscreen" },
  beforeEach: () => mockApi({ [members]: people }),
} satisfies Meta<typeof Dialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const CreateSpace: Story = { args: { form: createSpace } };
export const SaveError: Story = { args: { form: createSpace, fail: true } };
export const CreateAccount: Story = { args: { form: createAccount } };
