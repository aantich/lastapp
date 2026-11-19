import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Check,
  Download,
  ExternalLink,
  Pencil,
  Plus,
  RefreshCw,
  Rocket,
  Trash2,
} from "lucide-react";
import { Button, IconButton, LinkButton } from "../button";
import { Icon } from "../icon";

// button.tsx: Button (primary/secondary/ghost/danger/link, sm/md, loading)
// and IconButton, whose label is required and is its accessible name.
const meta = {
  title: "Primitives/Button",
  component: Button,
  args: { children: "Deploy release", variant: "primary" },
} satisfies Meta<typeof Button>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Playground: Story = {};

export const Variants: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button
        variant="primary"
        icon={<Icon icon={Rocket} size="lg" aria-hidden />}
      >
        Deploy r15
      </Button>
      <Button
        variant="secondary"
        icon={<Icon icon={Plus} size="lg" aria-hidden />}
      >
        Add field
      </Button>
      <Button variant="ghost">Cancel</Button>
      <Button
        variant="danger"
        icon={<Icon icon={Trash2} size="lg" aria-hidden />}
      >
        Retire release
      </Button>
      <Button variant="link">View history</Button>
    </div>
  ),
};

export const Sizes: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="primary" size="sm">
        Publish
      </Button>
      <Button variant="primary">Publish</Button>
      <Button size="sm" icon={<Icon icon={Pencil} size="xs" aria-hidden />}>
        Edit
      </Button>
      <Button icon={<Icon icon={Pencil} size="md" aria-hidden />}>Edit</Button>
    </div>
  ),
};

export const States: Story = {
  render: () => (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="primary" loading>
          Saving…
        </Button>
        <Button loading>Refreshing</Button>
        <Button variant="danger" loading>
          Removing
        </Button>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="primary"
          disabled
          icon={<Icon icon={Check} size="lg" aria-hidden />}
        >
          Save changes
        </Button>
        <Button disabled>Previous</Button>
        <Button variant="ghost" disabled>
          Cancel
        </Button>
        <Button variant="danger" disabled>
          Retire
        </Button>
      </div>
    </div>
  ),
};

export const IconButtons: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <IconButton
        label="Refresh deployments"
        icon={<Icon icon={RefreshCw} size="lg" aria-hidden />}
        tooltip
      />
      <IconButton
        label="Edit Deal"
        icon={<Icon icon={Pencil} size="lg" aria-hidden />}
        bordered
        tooltip
      />
      <IconButton
        label="Remove Lena Novak"
        tone="danger"
        icon={<Icon icon={Trash2} size="lg" aria-hidden />}
        tooltip="bottom"
      />
      <IconButton
        label="Refreshing"
        loading
        icon={<Icon icon={RefreshCw} size="lg" aria-hidden />}
      />
      <IconButton
        label="Edit (locked)"
        disabled
        icon={<Icon icon={Pencil} size="lg" aria-hidden />}
      />
      <IconButton
        label="Add"
        size="sm"
        icon={<Icon icon={Plus} size="sm" aria-hidden />}
      />
    </div>
  ),
};

// LinkButton: navigation that looks like a button (an anchor, never an
// action); `external` opens a new tab without an opener.
export const LinkButtons: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <LinkButton
        href="https://forecasting.northstar.example/"
        external
        variant="primary"
        trailingIcon={<Icon icon={ExternalLink} size="sm" aria-hidden />}
      >
        Open production App
      </LinkButton>
      <LinkButton
        href="#starter"
        icon={<Icon icon={Download} size="lg" aria-hidden />}
      >
        Get starter
      </LinkButton>
      <LinkButton href="#docs" variant="link">
        Read the guide
      </LinkButton>
    </div>
  ),
};
