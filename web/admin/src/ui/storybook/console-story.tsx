import type { ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import type { PasswordPolicy } from "../../api-types";
import { ConfigContext } from "../../console-config";
import { queryClient } from "../../query";
import { consoleConfig } from "./fixtures";

// The providers main.tsx gives every console screen, with a supplied
// password policy instead of the server's /v1/config response.
export function ConsoleStory({
  passwordPolicy,
  children,
}: {
  passwordPolicy: PasswordPolicy;
  children: ReactNode;
}) {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigContext.Provider
        value={{
          config: consoleConfig(passwordPolicy),
          refresh: async () => {},
        }}
      >
        {children}
      </ConfigContext.Provider>
    </QueryClientProvider>
  );
}

// The console form layout (FormModal's fields grid) for a control on its own.
export function StoryFields({ children }: { children: ReactNode }) {
  return (
    <div className="panel max-w-2xl">
      <div className="fields">{children}</div>
    </div>
  );
}

export function StoryField({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="form-field">
      <span className="field-label">{label}</span>
      {children}
      {hint && <small>{hint}</small>}
    </label>
  );
}
