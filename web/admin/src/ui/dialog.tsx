import { useId, type ReactNode } from "react";
import { X } from "lucide-react";
import { cx } from "./utils";
import { useDialogFocus } from "./use-dialog-focus";
import { Icon } from "./icon";

// The one modal shell. It is labelled by its title (and described by its
// description), sets aria-modal, focuses the first control (or the one marked
// data-autofocus), traps Tab, closes on Escape and on a press outside, and
// returns focus to the opener (useDialogFocus). FormModal, ConfirmDialog and
// the entity editor are wrappers on it. While `dismissable` is false (a save
// in flight) Escape, the backdrop and the close button do nothing.
export function Dialog({
  title,
  description,
  onClose,
  children,
  footer,
  role = "dialog",
  size = "md",
  closeLabel = "Close dialog",
  dismissable = true,
  closeOnBackdrop = true,
  className,
  titleId,
  describedBy,
}: {
  title: ReactNode;
  description?: ReactNode;
  onClose: () => void;
  children?: ReactNode;
  // Buttons, right-aligned; omit when children contain their own footer
  // (a form's submit row).
  footer?: ReactNode;
  role?: "dialog" | "alertdialog";
  size?: "sm" | "md" | "lg";
  closeLabel?: string;
  dismissable?: boolean;
  // A long form may prefer that a stray press outside keeps it open.
  closeOnBackdrop?: boolean;
  className?: string;
  // Fixed ids for screens/tests that reference them; generated otherwise.
  titleId?: string;
  describedBy?: string;
}) {
  const generated = useId();
  const labelId = titleId || `${generated}-title`,
    descriptionId = describedBy || `${generated}-description`;
  const close = () => {
    if (dismissable) onClose();
  };
  const ref = useDialogFocus(close);
  return (
    <div
      className="overlay"
      onMouseDown={(e) => {
        if (closeOnBackdrop && e.target === e.currentTarget) close();
      }}
    >
      <section
        ref={ref}
        className={cx("modal", size !== "md" && "ui-dialog-" + size, className)}
        tabIndex={-1}
        role={role}
        aria-modal="true"
        aria-labelledby={labelId}
        aria-describedby={description ? descriptionId : undefined}
      >
        <header>
          <div>
            <h2 id={labelId}>{title}</h2>
            {description && <p id={descriptionId}>{description}</p>}
          </div>
          <button
            type="button"
            className="icon"
            aria-label={closeLabel}
            disabled={!dismissable}
            onClick={close}
          >
            <Icon icon={X} size="2xl" aria-hidden />
          </button>
        </header>
        {children}
        {footer && <DialogFooter>{footer}</DialogFooter>}
      </section>
    </div>
  );
}

// The dialog's action row; place it inside a <form> to submit with Enter.
export function DialogFooter({ children }: { children: ReactNode }) {
  return <footer>{children}</footer>;
}

// Padded content area for dialogs whose body is not a form field grid.
export function DialogBody({ children }: { children: ReactNode }) {
  return <div className="ui-dialog-body">{children}</div>;
}
