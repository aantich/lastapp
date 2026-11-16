import { useEffect, useRef, type InputHTMLAttributes } from "react";
import { useConsoleConfig } from "./console-config";
import type { PasswordPolicy } from "./api-types";

export function usePasswordPolicy() {
  return useConsoleConfig().config.password_policy;
}

// Native minlength/maxlength count UTF-16 units; the API counts Unicode code
// points. Use one shared control with the server's limits for every NEW password.
// Login/current-password inputs deliberately do not enforce new-password policy.
export function NewPasswordInput(
  props: Omit<InputHTMLAttributes<HTMLInputElement>, "minLength" | "maxLength">,
) {
  const policy = usePasswordPolicy();
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (!input.current) return;
    const length = Array.from(input.current.value).length;
    input.current.setCustomValidity(
      length && (length < policy.min_length || length > policy.max_length)
        ? `Use a password of ${policy.length_hint}`
        : "",
    );
  }, [props.value, policy]);
  return <input {...props} ref={input} />;
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
