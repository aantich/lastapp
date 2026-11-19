import type { ReactNode } from "react";
import { CircleAlert, CircleCheck, Info, TriangleAlert, X } from "lucide-react";
import { cx } from "./utils";
import { Icon } from "./icon";

export type AlertTone = "danger" | "warning" | "info" | "success";

const icons = {
  danger: CircleAlert,
  warning: TriangleAlert,
  info: Info,
  success: CircleCheck,
};

// A message banner. Danger alerts keep the console's `.error` class (screens
// and tests rely on it) and are announced at once (role=alert); other tones
// are polite (role=status). `live={false}` renders a plain, unannounced
// message, e.g. a stored error inside a list row.
export function Alert({
  tone = "danger",
  title,
  children,
  actions,
  onDismiss,
  dismissLabel = "Dismiss",
  live = true,
  icon = true,
  as: As = "div",
  id,
  className,
}: {
  tone?: AlertTone;
  title?: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  onDismiss?: () => void;
  dismissLabel?: string;
  live?: boolean;
  icon?: boolean;
  as?: "div" | "p";
  // Lets a control name this message with aria-describedby.
  id?: string;
  className?: string;
}) {
  const Glyph = icons[tone];
  const role = !live ? undefined : tone === "danger" ? "alert" : "status";
  const Body = As === "p" ? "span" : "div";
  return (
    <As
      className={cx(
        tone === "danger" ? "error" : "tone-" + tone,
        "ui-alert",
        className,
      )}
      role={role}
      id={id}
    >
      {icon && <Icon icon={Glyph} size="lg" aria-hidden />}
      <Body className="ui-alert-body">
        {title && <strong>{title}</strong>}
        {children && <span>{children}</span>}
      </Body>
      {(actions || onDismiss) && (
        <span className="ui-alert-actions">
          {actions}
          {onDismiss && (
            <button
              type="button"
              className="icon"
              aria-label={dismissLabel}
              onClick={onDismiss}
            >
              <Icon icon={X} size="md" aria-hidden />
            </button>
          )}
        </span>
      )}
    </As>
  );
}

// A field-level validation message under a control. Link it to the control
// with aria-describedby (TextField does this).
export function InlineError({
  id,
  children,
}: {
  id?: string;
  children: ReactNode;
}) {
  return (
    <p className="ui-field-error" id={id}>
      <Icon icon={CircleAlert} size="xs" aria-hidden />
      {children}
    </p>
  );
}
