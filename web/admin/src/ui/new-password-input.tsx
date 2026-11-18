import { cx } from "./utils";
import { useEffect, useRef, type InputHTMLAttributes } from "react";
import { usePasswordPolicy } from "../console-config";

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
  return (
    <input {...props} className={cx("ui-input", props.className)} ref={input} />
  );
}
