import { AgentAccess } from "./agent-access";
import { PasswordScreen } from "./login";
import { Card, CardBody, CardHeader, Columns, FactList, Stack } from "./ui";
import type { Me } from "./api-types";

// The signed-in account's page, shared by the Platform and Tenant consoles:
// who you are, your password (or the SSO note) and your agents' access.
export function AccountPage({
  me,
  refreshMe,
}: {
  me: Me;
  refreshMe: () => Promise<void>;
}) {
  const password = me.auth_method === "password";
  return (
    <Stack>
      <Columns min={320}>
        <Card>
          <CardHeader title="Profile" />
          <CardBody>
            <FactList
              facts={[
                { label: "Name", value: me.display_name },
                { label: "Email", value: me.email },
                {
                  label: "Sign-in",
                  value: password ? "Email and password" : "Single sign-on",
                },
              ]}
            />
          </CardBody>
        </Card>
        {password ? (
          <PasswordScreen done={refreshMe} />
        ) : (
          <Card>
            <CardHeader
              title="Password"
              description="Manage your password with your single sign-on provider."
            />
          </Card>
        )}
      </Columns>
      <AgentAccess me={me} />
    </Stack>
  );
}
