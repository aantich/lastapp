import { AgentAccess } from "./agent-access";
import { PasswordScreen } from "./login";
import type { Me } from "./api-types";

// The signed-in account's page, shared by the Platform and Tenant consoles.
export function AccountPage({
  me,
  refreshMe,
}: {
  me: Me;
  refreshMe: () => Promise<void>;
}) {
  return (
    <>
      <AgentAccess me={me} />
      <section className="panel workspace-section">
        <p>
          {me.display_name} · {me.email}
        </p>
      </section>
      {me.auth_method === "password" ? (
        <PasswordScreen done={refreshMe} />
      ) : (
        <p className="panel workspace-section">
          Manage your password with your single sign-on provider.
        </p>
      )}
    </>
  );
}
