import { useState } from "react";
import { ConsoleShell } from "./console-shell";
import { parseTenantURL, useConsoleURL } from "./console-url";
import { Alert, Button, FormModal, type Modal } from "./ui";
import { useAppearance } from "./appearance";
import { readPreference } from "./preferences";
import { LiveRefresh, refreshQueries } from "./query";
import { useRouteState } from "./route-state";
import { useDeploymentStates } from "./deployment";
import { ConnectAgentDialog } from "./connect-agent";
import { text, type Me } from "./api-types";
import { tenantNav } from "./tenant/navigation";
import { useTenantQueries } from "./tenant/use-tenant-queries";
import { useTenantScreen } from "./tenant/use-tenant-screen";
import { paletteExamples, tenantCommands } from "./tenant/palette-commands";
import { TenantActions } from "./tenant/header-actions";
import { tenantCrumbs } from "./tenant/crumbs";
import { useSidebarPlaces } from "./tenant/use-sidebar-places";
import { SpacesLinks } from "./tenant/spaces-links";
import { TenantPages } from "./tenant/workspace-pages";
import { tenantContext } from "./tenant/tenant-context";
import { tenantAccountItems } from "./tenant/account-menu";
import {
  activeNavItem,
  pageTitle,
  peopleTabOf,
  routeDetail,
  shellHeading,
} from "./tenant/workspace-route";

// The Tenant console: resolves the Tenant and route, loads what every page
// shares (overview, capabilities and the open Space or App) and composes the
// pages under tenant/. Navigation comes from tenant/navigation.ts.
export function TenantWorkspace({
  me,
  appearanceControls,
  signOut,
  refreshMe,
}: {
  me: Me;
  appearanceControls: ReturnType<typeof useAppearance>;
  signOut: () => void;
  refreshMe: () => Promise<void>;
}) {
  const memberships = me.tenants || [],
    [url, navigate] = useConsoleURL(),
    parsed = parseTenantURL(url);
  const saved = readPreference("lastapp-tenant"),
    fallback = memberships.some((t) => t.id === saved)
      ? saved
      : memberships[0]?.id || "";
  const tenant = parsed.area === "tenants" ? parsed.tenant : fallback,
    base = "/v1/tenants/" + tenant;
  const [modal, setModal] = useRouteState<Modal | null>(url, null);
  const [connecting, setConnecting] = useState(false);
  const deploymentStates = useDeploymentStates();
  // Everything this workspace shows refreshes every 5 s while no form is open.
  // refetchInterval never overlaps requests and pauses in a hidden tab.
  const live = tenant && !modal ? 5000 : false;
  const queries = useTenantQueries(tenant, base, parsed, live);
  const { capsQuery, canSpaces, manager, activeAgents } = queries;
  const { t, error, dismiss, done } = useTenantScreen({
    me,
    url,
    navigate,
    parsed,
    tenant,
    base,
    queries,
    setModal,
  });
  const { route, profile, space, app, all, scoped, go } = t;
  const refresh = () => void refreshQueries();
  const connect = () => setConnecting(true);
  const { detail, detailReady } = routeDetail(route, space, app);
  const peopleTab = peopleTabOf(route, all);
  const title = pageTitle(route, detail, detailReady);
  const menu = tenantNav(t, { agents: activeAgents });
  const tenantName =
    text(profile.display_name) ||
    memberships.find((m) => m.id === tenant)?.display_name ||
    "";
  // The Space's entity designer is its own screen (the Entities section):
  // it draws its own header and list, so the Space header and tabs give way.
  const entityScreen = route.page === "space" && route.tab === "entities";
  const crumbs = tenantCrumbs(t, {
    tenantName,
    title,
    entityScreen,
    detailReady,
  });
  const paletteCommands = tenantCommands(t, {
    title,
    detailReady,
    failedState: deploymentStates.failed,
    menu,
    navigate,
    connect,
  });
  const actions = (
    <TenantActions
      t={t}
      detailReady={detailReady}
      refresh={refresh}
      connect={connect}
    />
  );
  const { context, section } = tenantContext({
    me,
    memberships,
    tenant,
    tenantName,
    navigate,
  });
  const places = useSidebarPlaces(t.spaces, scoped);
  const navActive = activeNavItem(route),
    heading = shellHeading(t, title, detailReady, entityScreen);
  return (
    <LiveRefresh value={live}>
      <ConsoleShell
        me={me}
        // The broadest fixed control role here, else the email.
        role={capsQuery.data?.control_roles[0]?.display_name || me.email}
        title={heading.title}
        description={heading.description}
        header={heading.header}
        nav={menu}
        active={navActive}
        navigate={go}
        section={section}
        context={context}
        places={
          tenant && canSpaces
            ? (rail, close) => (
                <SpacesLinks
                  rail={rail}
                  close={close}
                  items={places}
                  current={route.page === "spaces"}
                  active={navActive === "space" ? route.space : undefined}
                  go={go}
                />
              )
            : undefined
        }
        breadcrumbs={crumbs}
        commands={paletteCommands}
        paletteExamples={paletteExamples(paletteCommands)}
        actions={actions}
        topActions={
          <Button variant="primary" onClick={connect}>
            Connect agent
          </Button>
        }
        accountItems={tenantAccountItems(t, go)}
        appearanceControls={appearanceControls}
        account={() => go("account")}
        signOut={signOut}
      >
        {connecting && (
          <ConnectAgentDialog
            tenantName={tenantName}
            agentAccess={() => {
              setConnecting(false);
              go("agents");
            }}
            close={() => setConnecting(false)}
          />
        )}
        {error && (
          <Alert onDismiss={dismiss} dismissLabel="Dismiss error">
            {error}
          </Alert>
        )}
        <TenantPages
          t={t}
          url={url}
          refreshMe={refreshMe}
          detail={detail}
          detailReady={detailReady}
          entityScreen={entityScreen}
          peopleTab={peopleTab}
          title={title}
          actions={actions}
          manager={manager}
        />
        {modal && (
          <FormModal
            key={url + modal.title}
            modal={modal}
            lookups={{}}
            close={() => setModal(null)}
            done={done}
          />
        )}
      </ConsoleShell>
    </LiveRefresh>
  );
}
