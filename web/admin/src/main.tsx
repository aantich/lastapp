import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { LoaderCircle } from "lucide-react";
import "./style.css";
import { api, resetSessionRequests, sessionExpiredEvent } from "./api";
import { useAppearance } from "./appearance";
import { LoginScreen, PasswordScreen } from "./login";
import { TenantWorkspace } from "./tenant-workspace";
import { PlatformWorkspace } from "./platform-workspace";
import type { Me } from "./api-types";
import { ConsoleConfigProvider, useConsoleConfig } from "./console-config";
import { AppLaunch } from "./entity-designer/launch";
import { AgentConsent } from "./agent-access";
import { ConfirmHost } from "./confirm-dialog";
import { consoleURL, isConsolePath } from "./console-url";
import "./agent-access.css";
import { queryClient } from "./query";
function App() {
  const { refresh: refreshConfig } = useConsoleConfig();
  const appearanceControls = useAppearance();
  const [me, setMe] = useState<Me | null>(null),
    [auth, setAuth] = useState("loading"),
    [error, setError] = useState("");
  async function loadMe() {
    const who = await api<Me>("/v1/me");
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
    loadMe().catch((e) => {
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
  if (auth === "loading")
    return (
      <div className="boot">
        <LoaderCircle className="spin" />
        Opening Lastapp…
      </div>
    );
  if (auth === "out")
    return (
      <>
        {error && (
          <div className="error" role="alert">
            {error}
          </div>
        )}
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
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
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
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ConsoleConfigProvider>
        <App />
        <ConfirmHost />
      </ConsoleConfigProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
