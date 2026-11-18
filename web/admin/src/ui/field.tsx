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
// `hideLabel` keeps the label for assistive technology only, for a control
// whose row already says what it is (an expression's value).
export function FormField({
  label,
  hint,
  error,
  required,
  hideLabel,
  className,
  children,
}: {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  hideLabel?: boolean;
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
      {/* The required mark sits beside the label, not in it, so the label
          text (and a test's getByLabel) is exactly the field's name. */}
      <span className={cx("ui-field-label-row", hideLabel && "ui-sr-only")}>
        <label className="ui-field-label" htmlFor={id}>
          {label}
        </label>
        {required && <b aria-hidden>*</b>}
      </span>
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
  hideLabel?: boolean;
};

export const TextField = forwardRef<
  HTMLInputElement,
  FieldText &
    Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
      type?: "text" | "email" | "password" | "search" | "url" | "tel";
    }
>(function TextField(
  {
    label,
    hint,
    error,
    required,
    hideLabel,
    className,
    type = "text",
    ...rest
  },
  ref,
) {
  return (
    <FormField
      label={label}
      hint={hint}
      error={error}
      required={required}
      hideLabel={hideLabel}
    >
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
>(function Textarea(
  { label, hint, error, required, hideLabel, className, ...rest },
  ref,
) {
  return (
    <FormField
      label={label}
      hint={hint}
      error={error}
      required={required}
      hideLabel={hideLabel}
    >
      <textarea
        ref={ref}
        className={cx("ui-input", className)}
        required={required}
        {...rest}
      />
    </FormField>
  );
});
