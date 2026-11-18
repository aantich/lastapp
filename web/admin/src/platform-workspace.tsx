import { useEffect } from "react";
import {
  Building2,
  Users,
  ScrollText,
  Settings,
  Plus,
  RefreshCw,
} from "lucide-react";
import { ConsoleShell } from "./console-shell";
import { consoleURL, parsePlatformURL, useConsoleURL } from "./console-url";
import {
  Alert,
  Button,
  ContextLabel,
  ErrorBoundary,
  FormModal,
  IconButton,
  type Modal,
} from "./ui";
import { useAppearance } from "./appearance";
import { useAction } from "./use-action";
import { refreshQueries, useApiQuery } from "./query";
import { useRouteState, useScreenError } from "./route-state";
import { AccountPage } from "./account-page";
import { ActivityLog } from "./activity-log";
import { text, type Me, type Page, type TenantOverview } from "./api-types";
import type { PlatformScreen } from "./platform/screen";
import {
  addAdmin,
  createTenant,
  createUser,
  planForm,
  serverForm,
} from "./platform/forms";
import {
  PlatformSettings,
  PlatformTenant,
  TenantsPage,
  UsersPage,
} from "./platform/pages";

// The Platform console: resolves the route, loads plans and the open Tenant's
// overview and composes the pages under platform/.
const nav = [
  { id: "tenants", label: "Tenants", icon: Building2 },
  { id: "users", label: "Users", icon: Users },
  { id: "activity", label: "Activity", icon: ScrollText },
  { id: "settings", label: "Settings", icon: Settings },
];
export function PlatformWorkspace({
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
  const [path, navigate] = useConsoleURL(),
    route = parsePlatformURL(path),
    { page, tenant, tab, settingsTab } = route;
  const [modal, setModal] = useRouteState<Modal | null>(path, null);
  const plansQuery = useApiQuery<Page>("/admin/v1/plans"),
    overviewQuery = useApiQuery<TenantOverview>(
      tenant ? "/v1/tenants/" + tenant + "/overview" : null,
    ),
    plans = plansQuery.data?.items || [],
    overview: TenantOverview = (tenant && overviewQuery.data) || {};
  const refresh = () => void refreshQueries();
  const { error, setError, dismiss } = useScreenError(path, [
    plansQuery.error,
    overviewQuery.error,
  ]);
  const { busy, run: act } = useAction(path, {
    onSuccess: () => {},
    onError: setError,
  });
  function go(...page: string[]) {
    navigate(consoleURL.platform(...page));
  }
  useEffect(() => {
    if (!path.startsWith(consoleURL.platform()))
      navigate(consoleURL.platform("tenants"), true);
  }, [path, navigate]);
  const profile = overview.tenant || {};
  function done() {
    setModal(null);
  }
  const p: PlatformScreen = {
    me,
    route,
    tenant,
    overview,
    profile,
    plans,
    go,
    setModal,
    busy,
    act,
  };
  const title = tenant
    ? text(profile.display_name) || "Tenant"
    : page === "account"
      ? "My account"
      : nav.find((p) => p.id === page)?.label || "Tenants";
  const add = (label: string, run: () => void) => (
    <Button
      variant="primary"
      icon={<Plus size={16} aria-hidden />}
      onClick={run}
    >
      {label}
    </Button>
  );
  const addButton =
    tenant && tab === "administrators"
      ? add("Add administrator", () => addAdmin(p))
      : !tenant && page === "tenants"
        ? add("Create Tenant", () => createTenant(p))
        : page === "users"
          ? add("Create account", () => createUser(p))
          : page === "settings"
            ? settingsTab === "plans"
              ? add("Create plan", () => planForm(p))
              : add("Add database server", () => serverForm(p))
            : null;
  return (
    <ConsoleShell
      me={me}
      role="Platform operator"
      section={<ContextLabel value="Platform" />}
      title={title}
      nav={nav}
      active={page === "account" ? "" : page}
      navigate={go}
      appearanceControls={appearanceControls}
      account={() => go("account")}
      signOut={signOut}
      breadcrumbs={
        tenant
          ? [
              { label: "Platform", onClick: () => go("tenants") },
              { label: "Tenants", onClick: () => go("tenants") },
              { label: title },
            ]
          : [
              { label: "Platform", onClick: () => go("tenants") },
              { label: title },
            ]
      }
      actions={
        <>
          <IconButton
            label="Refresh"
            icon={<RefreshCw size={16} aria-hidden />}
            tooltip="bottom"
            onClick={refresh}
          />
          {addButton}
        </>
      }
    >
      {error && (
        <Alert onDismiss={dismiss} dismissLabel="Dismiss error">
          {error}
        </Alert>
      )}
      <ErrorBoundary key={path}>
        {tenant ? (
          <PlatformTenant p={p} />
        ) : page === "tenants" ? (
          <TenantsPage p={p} />
        ) : page === "users" ? (
          <UsersPage p={p} />
        ) : page === "activity" ? (
          <ActivityLog
            path={
              route.id
                ? "/v1/tenants/" + route.id + "/audit-events"
                : "/admin/v1/audit-events"
            }
          />
        ) : page === "settings" ? (
          <PlatformSettings p={p} />
        ) : page === "account" ? (
          <AccountPage me={me} refreshMe={refreshMe} />
        ) : null}
      </ErrorBoundary>
      {modal && (
        <FormModal
          key={path + modal.title}
          modal={modal}
          close={() => setModal(null)}
          done={done}
          lookups={{}}
        />
      )}
    </ConsoleShell>
  );
}
