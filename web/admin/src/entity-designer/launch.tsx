import { useState } from "react";
import { api } from "../api";
import { useApiQuery } from "../query";
import type { LaunchCode, LaunchInfo } from "../api-types";
import {
  Alert,
  Button,
  Card,
  FactList,
  Inline,
  PageHeader,
  Stack,
} from "../ui";

// The approval popup a browser App opens to connect (PKCE launch code): it
// names the App, its origin and Space, and posts the one-time code back to
// the opener's origin. The page shell is the console's standalone
// password-welcome layout.
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
      <Card className="welcome-card">
        <Stack>
          <PageHeader
            title={info ? "Connect " + info.name : "Open Lastapp connection"}
            description="Authorize this App to use your current Space permissions within its approved access ceiling."
          />
          {info && (
            <FactList
              facts={[
                { label: "App origin", value: info.origin, mono: true },
                { label: "Space", value: info.space },
              ]}
            />
          )}
          {shownError && <Alert as="p">{shownError}</Alert>}
          {done ? (
            <Alert tone="success">
              Connection sent to the App. You can close this window.
            </Alert>
          ) : (
            <Inline>
              <Button
                variant="primary"
                disabled={!info}
                onClick={() => {
                  if (!window.opener) {
                    setError("Open this connection from your App.");
                    return;
                  }
                  api<LaunchCode>(
                    "/v1/apps/" + encodeURIComponent(app) + "/launch",
                    "POST",
                    { nonce, challenge },
                  )
                    .then((v) => {
                      window.opener.postMessage(
                        {
                          type: "lastapp:launch",
                          code: v.code,
                          nonce: v.nonce,
                        },
                        v.origin,
                      );
                      setDone(true);
                    })
                    .catch((e) => setError(e.message));
                }}
              >
                Connect App
              </Button>
            </Inline>
          )}
        </Stack>
      </Card>
    </main>
  );
}
