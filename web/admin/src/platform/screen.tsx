import type { ReactNode } from "react";
import type { Modal } from "../ui";
import { text, type Item, type Me, type TenantOverview } from "../api-types";
import type { parsePlatformURL } from "../console-url";
import { status } from "../console-forms";

// What every Platform console page reads from the workspace: the route, the
// open Tenant, plans, navigation and the shared modal/action plumbing.
export type PlatformScreen = {
  me: Me;
  route: ReturnType<typeof parsePlatformURL>;
  tenant: string;
  overview: TenantOverview;
  profile: Item;
  plans: Item[];
  go: (...page: string[]) => void;
  setModal: (modal: Modal | null) => void;
  busy: boolean;
  act: (fn: () => Promise<unknown>) => Promise<void>;
};
export const columnsName: [string, (r: Item) => ReactNode] = [
  "Name",
  (r) => (
    <>
      <strong>{text(r.display_name)}</strong>
      {text(r.email) && <small>{text(r.email)}</small>}
    </>
  ),
];
export const columnsStatus: [string, (r: Item) => ReactNode] = [
  "Status",
  (r) => status(r.state),
];
