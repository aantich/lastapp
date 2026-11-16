import { useState } from "react";
import { Layers3, LoaderCircle, ArrowUpRight, LogOut } from "lucide-react";
import { api } from "./api";
import { useConsoleConfig } from "./console-config";
import { NewPasswordInput, usePasswordPolicy } from "./new-password-input";
import { errorMessage } from "./use-action";

export function LoginScreen({ done }: { done: () => Promise<void> }) {
  const [email, setEmail] = useState(""),
    [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const { config } = useConsoleConfig();
  return (
    <div className="welcome">
      <div className="welcome-card">
        <span className="brand-mark">
          <Layers3 size={30} />
        </span>
        <span className="eyebrow">LASTAPP</span>
        <h1>Welcome to your workspace.</h1>
        <p>Sign in to build and manage your company's Apps.</p>
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
          <label>
            Email
            <input
              type="email"
              autoComplete="username"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>
          <label>
            Password
            <input
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          {error && (
            <div className="error" role="alert">
              {error}
            </div>
          )}
          <button className="primary" disabled={busy}>
            {busy ? <LoaderCircle className="spin" size={18} /> : "Sign in"}
          </button>
        </form>
        {config.oidc_enabled && (
          <a
            className="secondary sso-link"
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
            href={
              "/auth/login?return_origin=" + encodeURIComponent(location.origin)
            }
          >
            Continue with SSO <ArrowUpRight size={16} />
          </a>
        )}
        <small>Use the account provided by your administrator.</small>
      </div>
      <div className="welcome-art">
        <div className="orbit o1" />
        <div className="orbit o2" />
        <div className="orbit o3" />
        <Layers3 size={100} />
        <span className="art-label">A foundation for every app.</span>
      </div>
    </div>
  );
}

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
  const Heading = required ? "h1" : "h2";
  return (
    <section
      className={
        required ? "welcome password-welcome" : "panel account-password"
      }
    >
      <div className="welcome-card">
        {required && (
          <span className="brand-mark">
            <Layers3 size={28} />
          </span>
        )}
        <Heading>
          {required ? "Set your own password" : "Change password"}
        </Heading>
        <p>
          {required
            ? "Replace your initial password before opening your workspace."
            : "Changing your password signs out your other sessions."}
        </p>
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
          <label>
            Current password
            <input
              type="password"
              autoComplete="current-password"
              required
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
            />
          </label>
          <label>
            New password
            <NewPasswordInput
              type="password"
              autoComplete="new-password"
              aria-label="New password"
              required
              value={next}
              onChange={(e) => setNext(e.target.value)}
            />
            <small>
              {policy.length_hint} Spaces and passphrases are welcome.
            </small>
          </label>
          <label>
            Confirm new password
            <input
              type="password"
              autoComplete="new-password"
              required
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
          </label>
          {success && <p role="status">{success}</p>}
          {error && (
            <div className="error" role="alert">
              {error}
            </div>
          )}
          <button className="primary" disabled={busy}>
            {busy ? "Saving…" : "Save password"}
          </button>
        </form>
        {signOut && (
          <button className="text-button" onClick={signOut}>
            <LogOut size={16} /> Sign out
          </button>
        )}
      </div>
    </section>
  );
}
