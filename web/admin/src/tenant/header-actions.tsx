import { Plus, RefreshCw, Users } from "lucide-react";
import { Button, IconButton, Icon } from "../ui";
import type { TenantScreen } from "./screen";
import { addDatabase, createApp, createSpace } from "./forms";
import { peopleVisible } from "./navigation";

// The current page's header actions: refresh, and the create/connect action
// the page offers to this account.
export function TenantActions({
  t,
  detailReady,
  refresh,
  connect,
}: {
  t: TenantScreen;
  detailReady: boolean;
  refresh: () => void;
  connect: () => void;
}) {
  const { route, caps, overview, all, scoped, go } = t;
  const canCreateApp =
    all("app.manage") || (caps["app.manage"]?.spaces || []).length > 0;
  return (
    <>
      {route.page !== "apps" && (
        <IconButton
          label="Refresh"
          icon={<Icon icon={RefreshCw} size="lg" aria-hidden />}
          tooltip="bottom"
          onClick={refresh}
        />
      )}
      {route.page === "apps" && canCreateApp && (overview.spaces ?? 0) > 0 && (
        <Button onClick={() => createApp(t)}>New app</Button>
      )}
      {route.page === "spaces" && all("space.manage") && (
        <Button
          variant="primary"
          icon={<Icon icon={Plus} size="lg" aria-hidden />}
          onClick={() => void createSpace(t)}
        >
          Create Space
        </Button>
      )}
      {route.page === "space" &&
        route.tab === "apps" &&
        scoped("app.manage") &&
        detailReady && (
          <Button
            variant="primary"
            icon={<Icon icon={Plus} size="lg" aria-hidden />}
            onClick={() => createApp(t)}
          >
            Create App
          </Button>
        )}
      {route.page === "space" && route.tab === "access" && peopleVisible(t) && (
        <Button
          icon={<Icon icon={Users} size="lg" aria-hidden />}
          onClick={() => go("people")}
        >
          Members and teams
        </Button>
      )}
      {route.page === "agents" && (
        <Button variant="primary" onClick={connect}>
          New connection
        </Button>
      )}
      {route.page === "settings" &&
        route.tab === "databases" &&
        all("database.manage") && (
          <Button variant="primary" onClick={() => void addDatabase(t)}>
            Add database
          </Button>
        )}
    </>
  );
}
