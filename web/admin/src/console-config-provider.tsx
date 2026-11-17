import { useCallback, useEffect, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiKey } from "./query";
import { ConfigContext, configPath, loadConsoleConfig } from "./console-config";
import { Alert, Button } from "./ui";

// Loads the console configuration before anything else renders and refreshes
// it whenever the window regains focus or is restored from the back/forward
// cache, so a changed server policy reaches open tabs.
export function ConsoleConfigProvider({ children }: { children: ReactNode }) {
  const query = useQuery({
    queryKey: apiKey(configPath),
    queryFn: ({ signal }) => loadConsoleConfig(signal),
  });
  const { refetch } = query;
  const refresh = useCallback(async () => {
    await refetch({ throwOnError: true });
  }, [refetch]);
  useEffect(() => {
    const reload = () => {
      void refetch();
    };
    window.addEventListener("focus", reload);
    window.addEventListener("pageshow", reload);
    return () => {
      window.removeEventListener("focus", reload);
      window.removeEventListener("pageshow", reload);
    };
  }, [refetch]);
  const config = query.data;
  const error = query.error?.message || "";
  return (
    <>
      {error && (
        <Alert
          actions={
            <Button size="sm" onClick={() => void refetch()}>
              Retry
            </Button>
          }
        >
          Cannot load server settings: {error}
        </Alert>
      )}
      {config ? (
        <ConfigContext.Provider value={{ config, refresh }}>
          {children}
        </ConfigContext.Provider>
      ) : (
        !error && <div className="boot">Opening Lastapp…</div>
      )}
    </>
  );
}
