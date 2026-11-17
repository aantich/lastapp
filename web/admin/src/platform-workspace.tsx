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
import { FormModal, type Modal } from "./form-modal";
import { useAppearance } from "./appearance";
import { useAction } from "./use-action";
import { refreshQueries, useApiQuery } from "./query";
import { useRouteState, useScreenError } from "./route-state";
import { AccountPage } from "./account-page";
import { ActivityLog } from "./activity-log";
import { ErrorBoundary } from "./error-boundary";
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
  const addButton =
    tenant && tab === "administrators" ? (
      <button className="primary" onClick={() => addAdmin(p)}>
        <Plus size={16} />
        Add administrator
      </button>
    ) : !tenant && page === "tenants" ? (
      <button className="primary" onClick={() => createTenant(p)}>
        <Plus size={16} />
        Create Tenant
      </button>
    ) : page === "users" ? (
      <button className="primary" onClick={() => createUser(p)}>
        <Plus size={16} />
        Create account
      </button>
    ) : page === "settings" ? (
      <button
        className="primary"
        onClick={() => (settingsTab === "plans" ? planForm(p) : serverForm(p))}
      >
        <Plus size={16} />
        {settingsTab === "plans" ? "Create plan" : "Add database server"}
      </button>
    ) : null;
  return (
    <ConsoleShell
      me={me}
      kind="Platform"
      title={title}
      nav={nav}
      active={page === "account" ? "" : page}
      navigate={go}
      appearanceControls={appearanceControls}
      account={() => go("account")}
      signOut={signOut}
      breadcrumbs={
        tenant ? (
          <>
            <button className="text-button" onClick={() => go("tenants")}>
              Tenants
            </button>
            <span> / {text(profile.display_name) || "Tenant"}</span>
          </>
        ) : (
          "Platform"
        )
      }
      actions={
        <>
          <button className="icon" aria-label="Refresh" onClick={refresh}>
            <RefreshCw size={16} />
          </button>
          {addButton}
        </>
      }
    >
      {error && (
        <div className="error" role="alert">
          {error}
          <button className="tiny" onClick={dismiss} aria-label="Dismiss error">
            ×
          </button>
        </div>
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
