import { useState } from "react";
import { api } from "../api";
import { useApiQuery } from "../query";
import type { LaunchCode, LaunchInfo } from "../api-types";
export function AppLaunch() {
  const [error, setError] = useState(""),
    [done, setDone] = useState(false);
  const q = new URLSearchParams(location.search);
  const app = q.get("app") || "",
    nonce = q.get("nonce") || "",
    challenge = q.get("challenge") || "";
  const launchQuery = useApiQuery<LaunchInfo>(
      "/v1/apps/" + encodeURIComponent(app) + "/launch-info",
    ),
    info = launchQuery.data || null,
    shownError = error || launchQuery.error?.message || "";
  return (
    <main className="password-welcome">
      <section className="welcome-card">
        <h1>{info ? "Connect " + info.name : "Open Lastapp connection"}</h1>
        {info && (
          <p>
            <strong>{info.origin}</strong>
            <br />
            Space: {info.space}
          </p>
        )}
        <p>
          Authorize this App to use your current Space permissions within its
          approved access ceiling.
        </p>
        {shownError && (
          <p className="error" role="alert">
            {shownError}
          </p>
        )}
        {done ? (
          <p>Connection sent to the App. You can close this window.</p>
        ) : (
          <button
            className="primary"
            disabled={!info}
            onClick={() => {
              if (!window.opener) {
                setError("Open this connection from your App.");
                return;
              }
              api<LaunchCode>(
                "/v1/apps/" + encodeURIComponent(app) + "/launch",
                "POST",
                {
                  nonce,
                  challenge,
                },
              )
                .then((v) => {
                  window.opener.postMessage(
                    { type: "lastapp:launch", code: v.code, nonce: v.nonce },
                    v.origin,
                  );
                  setDone(true);
                })
                .catch((e) => setError(e.message));
            }}
          >
            Connect App
          </button>
        )}
      </section>
    </main>
  );
}
