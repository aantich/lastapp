import { useEffect, useState } from "react";
import { api, resetSessionRequests, sessionExpiredEvent } from "./api";
import { useAppearance } from "./appearance";
import { LoginScreen } from "./login";
import { PasswordScreen } from "./password-screen";
import { TenantWorkspace } from "./tenant-workspace";
import { PlatformWorkspace } from "./platform-workspace";
import type { Me } from "./api-types";
import { useConsoleConfig } from "./console-config";
import { AppLaunch } from "./entity-designer/launch";
import { AgentConsent } from "./agent-access/agent-consent";
import { consoleURL, isConsolePath } from "./console-url";
import { queryClient } from "./query";
import { Alert, LoadingScreen } from "./ui";

// The console's session gate: loads the signed-in account, then shows sign-in,
// a required password change, the App launch or agent consent screen, or the
// Platform or Tenant workspace.
export function App() {
  const { refresh: refreshConfig } = useConsoleConfig();
  const appearanceControls = useAppearance();
  const [me, setMe] = useState<Me | null>(null),
    [auth, setAuth] = useState("loading"),
    [error, setError] = useState("");
  async function loadMe() {
    signedIn(await api<Me>("/v1/me"));
  }
  function signedIn(who: Me) {
    setMe(who);
    setAuth(who.password_change_required ? "password" : "in");
    if (
      !who.password_change_required &&
      location.pathname === consoleURL.root
    ) {
      try {
        const back = sessionStorage.getItem("lastapp-return-path");
        sessionStorage.removeItem("lastapp-return-path");
        if (isConsolePath(back)) {
          history.replaceState(null, "", back);
          window.dispatchEvent(new PopStateEvent("popstate"));
        }
      } catch {
        // sessionStorage can be unavailable (private mode, blocked storage)
      }
    }
  }
  function endSession() {
    resetSessionRequests();
    // Cached reads belong to the session that made them.
    queryClient.clear();
    setAuth("out");
    setMe(null);
    setError("");
  }
  useEffect(() => {
    window.addEventListener(sessionExpiredEvent, endSession);
    // State changes only in the response callbacks, not during the effect.
    api<Me>("/v1/me")
      .then(signedIn)
      .catch((e) => {
        endSession();
        if (e.status !== 401) setError(e.message);
      });
    return () => window.removeEventListener(sessionExpiredEvent, endSession);
  }, []);
  function signOut() {
    api("/auth/logout", "POST", {})
      .then(() => {
        endSession();
        history.replaceState(null, "", consoleURL.root);
        try {
          sessionStorage.removeItem("lastapp-return-path");
        } catch {
          // sessionStorage can be unavailable (private mode, blocked storage)
        }
      })
      .catch((e) => {
        if (e.status === 401) endSession();
        else setError(e.message);
      });
  }
  if (auth === "loading") return <LoadingScreen label="Opening Lastapp…" />;
  if (auth === "out")
    return (
      <>
        {error && <Alert>{error}</Alert>}
        <LoginScreen
          done={async () => {
            await refreshConfig();
            await loadMe();
          }}
        />
      </>
    );
  if (auth === "password")
    return <PasswordScreen required done={loadMe} signOut={signOut} />;
  if (location.pathname === consoleURL.launch) return <AppLaunch />;
  if (location.pathname === consoleURL.agentConsent)
    return <AgentConsent me={me!} />;
  return (
    <>
      {error && <Alert>{error}</Alert>}
      {me?.platform_admin ? (
        <PlatformWorkspace
          me={me}
          appearanceControls={appearanceControls}
          signOut={signOut}
          refreshMe={loadMe}
        />
      ) : (
        <TenantWorkspace
          me={me!}
          appearanceControls={appearanceControls}
          signOut={signOut}
          refreshMe={loadMe}
        />
      )}
    </>
  );
}
