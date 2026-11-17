import {
  cloneElement,
  forwardRef,
  useId,
  type InputHTMLAttributes,
  type ReactElement,
  type ReactNode,
  type TextareaHTMLAttributes,
} from "react";
import { cx } from "./utils";
import { InlineError } from "./alert";

type ControlProps = {
  id?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  "aria-required"?: boolean;
};

// Label, hint and error around one control, wired by id: the label names it,
// hint and error describe it, an error sets aria-invalid. Use it for any kit
// control (Select, NumberInput, NewPasswordInput) as well as plain inputs.
export function FormField({
  label,
  hint,
  error,
  required,
  className,
  children,
}: {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  className?: string;
  children: ReactElement<ControlProps>;
}) {
  const generated = useId(),
    id = children.props.id || generated;
  const hintId = hint ? `${id}-hint` : undefined,
    errorId = error ? `${id}-error` : undefined;
  const describedBy =
    [children.props["aria-describedby"], hintId, errorId]
      .filter(Boolean)
      .join(" ") || undefined;
  return (
    <div className={cx("ui-field", className)}>
      <label className="ui-field-label" htmlFor={id}>
        {label}
        {required && <b aria-hidden> *</b>}
      </label>
      {cloneElement(children, {
        id,
        "aria-describedby": describedBy,
        "aria-invalid": error ? true : undefined,
        "aria-required": required || undefined,
      })}
      {hint && <small id={hintId}>{hint}</small>}
      {error && <InlineError id={errorId}>{error}</InlineError>}
    </div>
  );
}

type FieldText = {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
};

export const TextField = forwardRef<
  HTMLInputElement,
  FieldText &
    Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
      type?: "text" | "email" | "password" | "search" | "url" | "tel";
    }
>(function TextField(
  { label, hint, error, required, className, type = "text", ...rest },
  ref,
) {
  return (
    <FormField label={label} hint={hint} error={error} required={required}>
      <input
        ref={ref}
        type={type}
        className={cx("ui-input", className)}
        required={required}
        {...rest}
      />
    </FormField>
  );
});

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  FieldText & TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ label, hint, error, required, className, ...rest }, ref) {
  return (
    <FormField label={label} hint={hint} error={error} required={required}>
      <textarea
        ref={ref}
        className={cx("ui-input", className)}
        required={required}
        {...rest}
      />
    </FormField>
  );
});
