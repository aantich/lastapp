import type { ReactNode } from "react";
import { Button, type Field, type Modal } from "../ui";
import {
  text,
  type Item,
  type Me,
  type TenantCapabilities,
  type TenantOverview,
} from "../api-types";
import { stateOptions, status as badge } from "../console-forms";

// What every Tenant workspace page reads from the workspace: the signed-in
// account, the Tenant and its loaded subjects, capability checks, navigation
// and the shared modal/action plumbing. The workspace builds it once per
// render; pages never fetch or decide these themselves.
export type TenantRoute = {
  page: string;
  space: string;
  app: string;
  tab: string;
  team: string;
};
export type TenantScreen = {
  me: Me;
  tenant: string;
  base: string;
  route: TenantRoute;
  caps: TenantCapabilities;
  overview: TenantOverview;
  profile: Item;
  space: Item;
  app: Item;
  // The sidebar's Spaces (the first page of the Tenant's Spaces).
  spaces: Item[];
  all: (action: string) => boolean;
  can: (action: string) => boolean;
  scoped: (action: string, spaceID?: string, appID?: string) => boolean;
  go: (page: string, v?: Partial<TenantRoute>) => void;
  openSpace: (row: Item) => void;
  openApp: (row: Item) => void;
  // Opens an entity in its Space's entity designer.
  openEntity: (space: string, entity: string) => void;
  edit: (row: Item, path: string, title: string, fields?: Field[]) => void;
  setModal: (modal: Modal | null) => void;
  setError: (message: string) => void;
  busy: boolean;
  act: (fn: () => Promise<unknown>) => Promise<void>;
};
export const states = stateOptions;
export const editFields: Field[] = [
  { key: "display_name", label: "Name", required: true },
  {
    key: "state",
    label: "Status",
    type: "select",
    options: states,
    required: true,
  },
];
export function computeField(dedicated: boolean): Field {
  return {
    key: "compute_mode",
    label: "Compute preference",
    type: "select",
    required: true,
    advanced: true,
    options: [
      { value: "shared", label: "Shared" },
      ...(dedicated ? [{ value: "dedicated", label: "Dedicated" }] : []),
    ],
  };
}
export const options = (items: Item[], key = "id", label = "display_name") =>
  items.map((x) => ({ value: text(x[key]), label: text(x[label]) }));
export const nameColumn: [string, (r: Item) => ReactNode] = [
    "Name",
    (r) => <strong>{text(r.display_name)}</strong>,
  ],
  statusColumn: [string, (r: Item) => ReactNode] = [
    "Status",
    (r) => badge(r.state),
  ],
  createdColumn: [string, (r: Item) => ReactNode] = [
    "Created",
    (r) => new Date(text(r.created_at)).toLocaleDateString(),
  ];

// The Open action of a list row (Apps, Spaces).
export const openAction = (open: (row: Item) => void) => (row: Item) => (
  <Button size="sm" variant="ghost" onClick={() => open(row)}>
    Open
  </Button>
);
