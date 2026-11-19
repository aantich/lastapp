import { useState } from "react";
import { Layers3, ArrowUpRight } from "lucide-react";
import { api } from "./api";
import { useConsoleConfig } from "./console-config";
import { Alert, AuthLayout, Button, LinkButton, Stack, TextField } from "./ui";
import { errorMessage } from "./use-action";

export { PasswordScreen } from "./password-screen";

export function LoginScreen({ done }: { done: () => Promise<void> }) {
  const [email, setEmail] = useState(""),
    [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const { config } = useConsoleConfig();
  return (
    <AuthLayout
      mark={<Layers3 size={24} />}
      eyebrow="Lastapp"
      title="Welcome to your workspace."
      description="Sign in to build and manage your company's Apps."
      footer={
        <>
          {config.oidc_enabled && (
            <LinkButton
              href={
                "/auth/login?return_origin=" +
                encodeURIComponent(location.origin)
              }
              trailingIcon={<ArrowUpRight size={16} aria-hidden />}
              onClick={() => {
                try {
                  sessionStorage.setItem(
                    "lastapp-return-path",
                    location.pathname + location.search,
                  );
                } catch {
                  // sessionStorage can be unavailable (private mode, blocked storage)
                }
              }}
            >
              Continue with SSO
            </LinkButton>
          )}
          <small>Use the account provided by your administrator.</small>
        </>
      }
    >
      <form
        className="login-form"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            await api("/auth/login/password", "POST", { email, password });
            setPassword("");
            await done();
          } catch (e) {
            setError(errorMessage(e));
          } finally {
            setBusy(false);
          }
        }}
      >
        <Stack gap="md">
          <TextField
            label="Email"
            type="email"
            autoComplete="username"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <TextField
            label="Password"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          {error && <Alert>{error}</Alert>}
          <Button variant="primary" type="submit" loading={busy}>
            Sign in
          </Button>
        </Stack>
      </form>
    </AuthLayout>
  );
}
