import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { api } from "./api";
import { errorMessage } from "./use-action";

export type PasswordPolicy = {
  min_length: number;
  max_length: number;
  length_hint: string;
};
type ConsoleConfig = {
  development_policy: { max_release_microapps: number };
  oidc_enabled: boolean;
  password_policy: PasswordPolicy;
  role_catalog: { id: string; code: string }[];
};
const ConfigContext = createContext<{
  config: ConsoleConfig;
  refresh: () => Promise<void>;
} | null>(null);

export function ConsoleConfigProvider({ children }: { children: ReactNode }) {
  const [config, setConfig] = useState<ConsoleConfig | null>(null);
  const [error, setError] = useState("");
  const request = useRef(0);
  const refresh = useCallback(async () => {
    const current = ++request.current;
    try {
      const next: ConsoleConfig = await api("/v1/config");
      const policy = next.password_policy;
      if (
        !policy ||
        !Number.isSafeInteger(policy.min_length) ||
        !Number.isSafeInteger(policy.max_length) ||
        policy.min_length < 1 ||
        policy.max_length < policy.min_length ||
        !policy.length_hint
      ) {
        throw new Error("The server did not provide a valid password policy.");
      }
      if (request.current === current) {
        setConfig(next);
        setError("");
      }
    } catch (e) {
      if (request.current === current) setError(errorMessage(e));
      throw e;
    }
  }, []);
  useEffect(() => {
    const reload = () => {
      void refresh().catch(() => {});
    };
    reload();
    window.addEventListener("focus", reload);
    window.addEventListener("pageshow", reload);
    return () => {
      request.current++;
      window.removeEventListener("focus", reload);
      window.removeEventListener("pageshow", reload);
    };
  }, [refresh]);
  return (
    <>
      {error && (
        <div className="error" role="alert">
          Cannot load server settings: {error}{" "}
          <button
            className="secondary"
            onClick={() => {
              void refresh().catch(() => {});
            }}
          >
            Retry
          </button>
        </div>
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

export function useConsoleConfig() {
  const value = useContext(ConfigContext);
  if (!value) throw new Error("ConsoleConfigProvider is required.");
  return value;
}

export function useBuiltinRoles() {
  const { config } = useConsoleConfig();
  const find = (code: string) => {
    const role = config.role_catalog.find((r) => r.code === code);
    if (!role) throw new Error("Missing protected role: " + code);
    return role.id;
  };
  return {
    platform: find("platform.superadmin"),
    tenant: find("tenant.admin"),
    teamAdmin: find("team.admin"),
    reviewer: find("team.reviewer"),
  };
}
