import type { Meta, StoryObj } from "@storybook/react-vite";
import { ArrowUpRight, Layers3, LogOut } from "lucide-react";
import { AuthLayout } from "../auth-layout";
import { Button, LinkButton } from "../button";
import { TextField } from "../field";
import { Stack } from "../layout";

// auth-layout.tsx: the signed-out and onboarding frame (sign in, set your own
// password) — one card centred on the canvas, outside the console shell.
const meta = {
  title: "Composites/AuthLayout",
  parameters: { layout: "fullscreen" },
} satisfies Meta;
export default meta;
type Story = StoryObj<typeof meta>;

export const SignIn: Story = {
  render: () => (
    <AuthLayout
      mark={<Layers3 size={24} />}
      eyebrow="Lastapp"
      title="Welcome to your workspace."
      description="Sign in to build and manage Northstar's Apps."
      footer={
        <>
          <LinkButton
            href="#sso"
            trailingIcon={<ArrowUpRight size={16} aria-hidden />}
          >
            Continue with SSO
          </LinkButton>
          <small>Use the account provided by your administrator.</small>
        </>
      }
    >
      <form onSubmit={(e) => e.preventDefault()}>
        <Stack gap="md">
          <TextField
            label="Email"
            type="email"
            autoComplete="username"
            defaultValue="ava.chen@northstar.example"
          />
          <TextField
            label="Password"
            type="password"
            autoComplete="current-password"
          />
          <Button variant="primary" type="submit">
            Sign in
          </Button>
        </Stack>
      </form>
    </AuthLayout>
  ),
};

export const Onboarding: Story = {
  render: () => (
    <AuthLayout
      mark={<Layers3 size={24} />}
      title="Set your own password"
      description="Replace your initial password before opening your workspace."
      footer={
        <Button variant="link" icon={<LogOut size={16} aria-hidden />}>
          Sign out
        </Button>
      }
    >
      <form onSubmit={(e) => e.preventDefault()}>
        <Stack gap="md">
          <TextField label="Current password" type="password" />
          <TextField
            label="New password"
            type="password"
            hint="12–128 characters. Spaces and passphrases are welcome."
          />
          <TextField label="Confirm new password" type="password" />
          <Button variant="primary" type="submit">
            Save password
          </Button>
        </Stack>
      </form>
    </AuthLayout>
  ),
};
