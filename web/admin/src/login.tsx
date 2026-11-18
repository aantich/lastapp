import { useState } from "react";
import { Layers3, ArrowUpRight, LogOut } from "lucide-react";
import { api } from "./api";
import { useConsoleConfig, usePasswordPolicy } from "./console-config";
import {
  Alert,
  AuthLayout,
  Button,
  Card,
  CardBody,
  CardHeader,
  FormField,
  Inline,
  LinkButton,
  NewPasswordInput,
  Stack,
  TextField,
} from "./ui";
import { errorMessage } from "./use-action";

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

// Changes the signed-in account's password: the first-login "set your own
// password" step (required) or the card on the account page. The length
// rule comes only from the server policy (GET /v1/config).
export function PasswordScreen({
  required = false,
  done,
  signOut,
}: {
  required?: boolean;
  done: () => Promise<void>;
  signOut?: () => void;
}) {
  const [current, setCurrent] = useState(""),
    [next, setNext] = useState(""),
    [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const policy = usePasswordPolicy();
  const form = (
    <form
      className="login-form"
      onSubmit={async (e) => {
        e.preventDefault();
        if (next !== confirm) {
          setError("The new passwords do not match.");
          return;
        }
        setBusy(true);
        setError("");
        setSuccess("");
        try {
          await api("/auth/password", "POST", {
            current_password: current,
            new_password: next,
          });
          setCurrent("");
          setNext("");
          setConfirm("");
          await done();
          setSuccess("Password changed.");
        } catch (e) {
          setError(errorMessage(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <Stack gap="md">
        <TextField
          label="Current password"
          type="password"
          autoComplete="current-password"
          required
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
        />
        <FormField
          label="New password"
          required
          hint={`${policy.length_hint} Spaces and passphrases are welcome.`}
        >
          <NewPasswordInput
            type="password"
            autoComplete="new-password"
            required
            value={next}
            onChange={(e) => setNext(e.target.value)}
          />
        </FormField>
        <TextField
          label="Confirm new password"
          type="password"
          autoComplete="new-password"
          required
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
        />
        {success && <Alert tone="success">{success}</Alert>}
        {error && <Alert>{error}</Alert>}
        <Inline>
          <Button variant="primary" type="submit" loading={busy}>
            Save password
          </Button>
        </Inline>
      </Stack>
    </form>
  );
  if (!required)
    return (
      <Card>
        <CardHeader
          title="Change password"
          description="Changing your password signs out your other sessions."
        />
        <CardBody>{form}</CardBody>
      </Card>
    );
  return (
    <AuthLayout
      className="password-welcome"
      cardClassName="welcome-card"
      mark={<Layers3 size={24} />}
      title="Set your own password"
      description="Replace your initial password before opening your workspace."
      footer={
        signOut && (
          <Button
            variant="link"
            icon={<LogOut size={16} aria-hidden />}
            onClick={signOut}
          >
            Sign out
          </Button>
        )
      }
    >
      {form}
    </AuthLayout>
  );
}
