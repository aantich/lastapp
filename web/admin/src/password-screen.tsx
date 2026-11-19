import { useState } from "react";
import { Layers3, LogOut } from "lucide-react";
import { api } from "./api";
import { usePasswordPolicy } from "./console-config";
import {
  Alert,
  AuthLayout,
  Button,
  Card,
  CardBody,
  CardHeader,
  FormField,
  Inline,
  NewPasswordInput,
  Stack,
  TextField,
} from "./ui";
import { errorMessage } from "./use-action";

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
