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
      <Button variant="primary" icon={<Rocket size={16} aria-hidden />}>
        Deploy r15
      </Button>
      <Button variant="secondary" icon={<Plus size={16} aria-hidden />}>
        Add field
      </Button>
      <Button variant="ghost">Cancel</Button>
      <Button variant="danger" icon={<Trash2 size={16} aria-hidden />}>
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
      <Button size="sm" icon={<Pencil size={13} aria-hidden />}>
        Edit
      </Button>
      <Button icon={<Pencil size={15} aria-hidden />}>Edit</Button>
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
          icon={<Check size={16} aria-hidden />}
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
        icon={<RefreshCw size={16} aria-hidden />}
        tooltip
      />
      <IconButton
        label="Edit Deal"
        icon={<Pencil size={16} aria-hidden />}
        bordered
        tooltip
      />
      <IconButton
        label="Remove Lena Novak"
        tone="danger"
        icon={<Trash2 size={16} aria-hidden />}
        tooltip="bottom"
      />
      <IconButton
        label="Refreshing"
        loading
        icon={<RefreshCw size={16} aria-hidden />}
      />
      <IconButton
        label="Edit (locked)"
        disabled
        icon={<Pencil size={16} aria-hidden />}
      />
      <IconButton label="Add" size="sm" icon={<Plus size={14} aria-hidden />} />
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
        trailingIcon={<ExternalLink size={14} aria-hidden />}
      >
        Open production App
      </LinkButton>
      <LinkButton href="#starter" icon={<Download size={16} aria-hidden />}>
        Get starter
      </LinkButton>
      <LinkButton href="#docs" variant="link">
        Read the guide
      </LinkButton>
    </div>
  ),
};
