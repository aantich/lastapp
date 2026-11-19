import { GitBranch } from "lucide-react";
import { workspacesPath } from "../workspaces";
import { statusTone, Icon } from "../ui";
import { useApiQueries } from "../query";
import { text, type Item, type Page } from "../api-types";
import type { TenantScreen } from "./screen";

// Each listed Space's development workspaces this account may manage,
// tagged "workspace" under their Space; Spaces themselves carry no tag.
// Every row keeps its status dot.
export function useSidebarPlaces(
  sidebarSpaces: Item[],
  scoped: TenantScreen["scoped"],
) {
  const workspaceParents = sidebarSpaces
    .map((sp) => text(sp.id))
    .filter((id) => scoped("workspace.manage", id, ""));
  const workspaceQueries = useApiQueries<Page>(
    workspaceParents.map(workspacesPath),
  );
  return sidebarSpaces.flatMap((sp) => [
    {
      id: text(sp.id),
      label: text(sp.display_name),
      tone: statusTone(text(sp.database_state || sp.state)),
    },
    ...(
      workspaceQueries[workspaceParents.indexOf(text(sp.id))]?.data?.items || []
    )
      .filter(
        (w) =>
          !!w.backing_space_id &&
          !["archived", "deleted"].includes(text(w.state)),
      )
      .map((w) => ({
        id: text(w.backing_space_id),
        label: text(w.display_name),
        tone: statusTone(text(w.state)),
        tag: (
          <>
            <Icon icon={GitBranch} size="2xs" aria-hidden />
            workspace
          </>
        ),
      })),
  ]);
}
