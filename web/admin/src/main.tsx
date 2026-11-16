import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { LoaderCircle } from "lucide-react";
import "./style.css";
import { api, resetSessionRequests, sessionExpiredEvent } from "./api";
import { useAppearance } from "./appearance";
import { LoginScreen, PasswordScreen } from "./login";
import { TenantWorkspace } from "./tenant-workspace";
import { PlatformWorkspace } from "./platform-workspace";
import type { Item } from "./console-forms";
import { ConsoleConfigProvider, useConsoleConfig } from "./console-config";
import { AppLaunch } from "./entity-designer/launch";
import { AgentConsent } from "./agent-access";
import { ConfirmHost } from "./confirm-dialog";
import "./agent-access.css";
function App() {
  const { refresh: refreshConfig } = useConsoleConfig();
  const appearanceControls = useAppearance();
  const [me, setMe] = useState<Item | null>(null),
    [auth, setAuth] = useState("loading"),
    [error, setError] = useState("");
  async function loadMe() {
    const who = await api("/v1/me");
    setMe(who);
    setAuth(who.password_change_required ? "password" : "in");
    if (!who.password_change_required && location.pathname === "/console/") {
      try {
        const back = sessionStorage.getItem("lastapp-return-path");
        sessionStorage.removeItem("lastapp-return-path");
        if (back?.startsWith("/console/") && !back.startsWith("/console//")) {
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
        history.replaceState(null, "", "/console/");
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
  if (location.pathname === "/console/launch") return <AppLaunch />;
  if (location.pathname === "/console/agent-consent")
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
    <ConsoleConfigProvider>
      <App />
      <ConfirmHost />
    </ConsoleConfigProvider>
  </React.StrictMode>,
);
