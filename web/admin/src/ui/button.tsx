import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { Tooltip } from "./tooltip";
import { cx } from "./utils";

export type ButtonVariant =
  "primary" | "secondary" | "ghost" | "danger" | "link";
export type ButtonSize = "sm" | "md";

// The console button classes. `danger` keeps the established
// "secondary danger" pair that screens and tests match on.
const variantClass: Record<ButtonVariant, string> = {
  primary: "primary",
  secondary: "secondary",
  ghost: "ui-ghost",
  danger: "secondary danger",
  link: "text-button",
};

type BaseProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> & {
  size?: ButtonSize;
  // Shows a spinner, disables the button and marks it aria-busy.
  loading?: boolean;
};

export type ButtonProps = BaseProps & {
  variant?: ButtonVariant;
  // Leading icon (lucide element); replaced by the spinner while loading.
  icon?: ReactNode;
  children?: ReactNode;
};

// A text button. Defaults to type="button"; pass type="submit" in forms.
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  function Button(
    {
      variant = "secondary",
      size = "md",
      loading = false,
      icon,
      className,
      disabled,
      type = "button",
      children,
      ...rest
    },
    ref,
  ) {
    return (
      <button
        ref={ref}
        type={type}
        className={cx(
          variantClass[variant],
          size === "sm" && "ui-sm",
          className,
        )}
        disabled={disabled || loading}
        aria-busy={loading || undefined}
        {...rest}
      >
        {loading ? (
          <LoaderCircle
            className="spin"
            size={size === "sm" ? 14 : 16}
            aria-hidden
          />
        ) : (
          icon
        )}
        {children}
      </button>
    );
  },
);

export type IconButtonProps = BaseProps & {
  // Required: the accessible name, also shown as the tooltip.
  label: string;
  icon: ReactNode;
  tone?: "default" | "danger";
  bordered?: boolean;
  // Also show the label as a tooltip on hover and keyboard focus.
  tooltip?: boolean | "top" | "right" | "bottom";
};

// An icon-only button. The label is its accessible name, never optional.
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  function IconButton(
    {
      label,
      icon,
      tone = "default",
      bordered,
      tooltip = false,
      size = "md",
      loading,
      disabled,
      className,
      type = "button",
      ...rest
    },
    ref,
  ) {
    const button = (
      <button
        ref={ref}
        type={type}
        aria-label={label}
        className={cx(
          "icon",
          tone === "danger" && "danger",
          bordered && "ui-bordered",
          size === "sm" && "ui-sm",
          className,
        )}
        disabled={disabled || loading}
        aria-busy={loading || undefined}
        {...rest}
      >
        {loading ? (
          <LoaderCircle className="spin" size={16} aria-hidden />
        ) : (
          icon
        )}
      </button>
    );
    if (!tooltip) return button;
    return (
      <Tooltip
        content={label}
        side={tooltip === true ? "top" : tooltip}
        labelOnly
      >
        {button}
      </Tooltip>
    );
  },
);
