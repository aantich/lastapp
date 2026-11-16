import { createContext, useContext } from "react";
import { api } from "./api";
import type { ConsoleConfig, PasswordPolicy } from "./api-types";

export const configPath = "/v1/config";

// Reads the public console configuration and rejects a response without a
// usable password policy: the UI never falls back to its own limits.
export async function loadConsoleConfig(signal?: AbortSignal) {
  const next = await api<ConsoleConfig>(
    configPath,
    "GET",
    undefined,
    undefined,
    signal,
  );
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
  return next;
}

export const ConfigContext = createContext<{
  config: ConsoleConfig;
  refresh: () => Promise<void>;
} | null>(null);

export function useConsoleConfig() {
  const value = useContext(ConfigContext);
  if (!value) throw new Error("ConsoleConfigProvider is required.");
  return value;
}

export function usePasswordPolicy() {
  return useConsoleConfig().config.password_policy;
}

export function generatePassword(policy: PasswordPolicy) {
  // Prefer 40 hex characters of random entropy, bounded by the server policy.
  const length = Math.min(policy.max_length, Math.max(policy.min_length, 40));
  return Array.from(
    crypto.getRandomValues(new Uint8Array(Math.ceil(length / 2))),
    (byte) => byte.toString(16).padStart(2, "0"),
  )
    .join("")
    .slice(0, length);
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
