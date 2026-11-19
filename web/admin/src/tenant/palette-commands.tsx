import {
  Activity as ActivityIcon,
  AppWindow,
  Bot,
  Database,
  FolderOpen,
  RotateCcw,
} from "lucide-react";
import { consoleURL, useConsoleURL } from "../console-url";
import type { Command, NavItem } from "../ui";
import { cachedData, cachedRows } from "../query";
import { activityQuery, auditActors } from "../actors";
import { retryDeployment } from "../deployment";
import { text, type Deployment, type Item } from "../api-types";
import type { TenantScreen } from "./screen";
import { peopleVisible, sectionLabel, tenantTabs } from "./navigation";
import { Icon } from "../ui";

// The palette's commands beyond the sidebar sections: entities, Apps and
// Spaces this session has already loaded, actions with a real operation
// (retrying a failed deployment Home has shown, connecting an agent), the
// Activity filters, this page's tabs and the other sections' tabs. Nothing
// is fetched for the palette.
export function tenantCommands(
  t: TenantScreen,
  {
    title,
    detailReady,
    failedState,
    menu,
    navigate,
    connect,
  }: {
    title: string;
    detailReady: boolean;
    // The failure state of a production deployment.
    failedState: string;
    menu: NavItem[];
    navigate: ReturnType<typeof useConsoleURL>[1];
    connect: () => void;
  },
): Command[] {
  const { route, space, overview, base, tenant, scoped, go } = t;
  const subject = route.page === "space" || route.page === "app" ? title : "";
  const tabCommands = (page: string, label: string, group: string) =>
    tenantTabs(page, t, space).map(({ id, label: tab }) => ({
      id: `tab:${page}:${id}`,
      label: `${label} › ${String(tab)}`,
      group,
      run: () => go(page, { space: route.space, app: route.app, tab: id }),
    }));
  const apps = new Map<string, Item>();
  for (const a of [
    ...(overview.recent_apps || []),
    ...cachedRows((p) => p === base + "/apps"),
  ])
    apps.set(text(a.id), a);
  const entities = cachedRows((p) => /^\/v1\/spaces\/[^/]+\/entities$/.test(p));
  const failed = cachedData<{ deployment: Deployment | null }>((p) =>
    /^\/v1\/apps\/[^/]+\/deployment$/.test(p),
  ).flatMap(([path, data]) => {
    const a = apps.get(path.split("/")[3]),
      d = data?.deployment;
    return a &&
      d?.state === failedState &&
      scoped("release.manage", text(a.space_id), text(a.id))
      ? [{ a, d }]
      : [];
  });
  return [
    ...entities.map((e) => ({
      id: "entity:" + text(e.id),
      label: `Open ${text(e.label)} entity`,
      group: "Entities",
      keywords: text(e.entity_key),
      hint: text(e.entity_key),
      icon: <Icon icon={Database} size="lg" />,
      run: () => t.openEntity(text(e.space_id), text(e.id)),
    })),
    ...[...apps.values()].map((a) => ({
      id: "app:" + text(a.id),
      label: `Open ${text(a.display_name)}`,
      group: "Apps",
      keywords: text(a.space_name),
      hint: text(a.space_name),
      icon: <Icon icon={AppWindow} size="lg" />,
      run: () => t.openApp(a),
    })),
    ...cachedRows((p) => p === base + "/spaces").map((sp) => ({
      id: "space:" + text(sp.id),
      label: `Open ${text(sp.display_name)}`,
      group: "Spaces",
      icon: <Icon icon={FolderOpen} size="lg" />,
      run: () => t.openSpace(sp),
    })),
    ...failed.map(({ a, d }) => ({
      id: "retry:" + d.id,
      label: `Retry ${text(a.display_name)} deploy`,
      group: "Action",
      keywords: d.release_name,
      icon: <Icon icon={RotateCcw} size="lg" />,
      run: () => retryDeployment(d, t.setModal),
    })),
    {
      id: "action:connect-agent",
      label: "Connect a new agent",
      group: "Action",
      icon: <Icon icon={Bot} size="lg" />,
      run: connect,
    },
    ...(menu.some((m) => m.id === "activity")
      ? auditActors
          .filter((a) => a.value)
          .map((a) => ({
            id: "activity:" + a.value,
            label: `Activity · ${a.label.toLowerCase()} only`,
            group: "Activity",
            icon: <Icon icon={ActivityIcon} size="lg" />,
            run: () =>
              navigate(
                consoleURL.tenant(tenant, "activity") + activityQuery(a.value),
              ),
          }))
      : []),
    ...(subject && detailReady
      ? tabCommands(route.page, subject, "This page")
      : []),
    ...["people", "settings"]
      .filter((page) =>
        page === "people" ? peopleVisible(t) : menu.some((m) => m.id === page),
      )
      .flatMap((page) =>
        tabCommands(page, sectionLabel(page) || page, "Sections"),
      ),
  ];
}

// Words that find real commands: an entity's key, "deploy" when a retry is
// offered, "agent".
export const paletteExamples = (commands: Command[]) =>
  [
    commands.find((c) => c.group === "Entities")?.keywords,
    commands.some((c) => c.id.startsWith("retry:")) && "deploy",
    "agent",
  ].filter((w): w is string => !!w);
