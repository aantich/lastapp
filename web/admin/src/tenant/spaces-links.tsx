import { SidebarLinks, type SidebarLink } from "../ui";
import type { TenantScreen } from "./screen";

// The sidebar's Spaces group: its heading opens the Spaces list, a row its
// Space. `active` is the highlighted Space; the shell's `close` runs after
// either choice.
export function SpacesLinks({
  rail,
  close,
  items,
  current,
  active,
  go,
}: {
  rail: boolean;
  close: () => void;
  items: SidebarLink[];
  // The Spaces list is the page shown.
  current: boolean;
  active: string | undefined;
  go: TenantScreen["go"];
}) {
  return (
    <SidebarLinks
      label="Spaces"
      collapsed={rail}
      title={
        <button
          type="button"
          className="ui-nav-section ui-section-trigger"
          aria-current={current ? "page" : undefined}
          onClick={() => {
            go("spaces");
            close();
          }}
        >
          Spaces
        </button>
      }
      items={items}
      active={active}
      onSelect={(id) => {
        go("space", { space: id });
        close();
      }}
    />
  );
}
