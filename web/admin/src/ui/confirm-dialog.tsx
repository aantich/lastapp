import { useSyncExternalStore } from "react";
import { X } from "lucide-react";
import { useDialogFocus } from "./use-dialog-focus";
import {
  pendingConfirm,
  subscribeConfirm,
  type ConfirmOptions,
} from "./confirm";

export function ConfirmDialog({
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  danger,
  onResult,
}: ConfirmOptions & { onResult: (confirmed: boolean) => void }) {
  const ref = useDialogFocus(() => onResult(false));
  return (
    <div
      className="overlay"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onResult(false);
      }}
    >
      <section
        ref={ref}
        className="modal confirm-modal"
        tabIndex={-1}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby={message ? "confirm-message" : undefined}
      >
        <header>
          <div>
            <h2 id="confirm-title">{title}</h2>
            {message && <p id="confirm-message">{message}</p>}
          </div>
          <button
            className="icon"
            aria-label="Close dialog"
            onClick={() => onResult(false)}
          >
            <X size={19} />
          </button>
        </header>
        <footer>
          <button
            type="button"
            className="secondary"
            data-autofocus={danger || undefined}
            onClick={() => onResult(false)}
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            className={danger ? "secondary danger" : "primary"}
            data-autofocus={!danger || undefined}
            onClick={() => onResult(true)}
          >
            {confirmLabel}
          </button>
        </footer>
      </section>
    </div>
  );
}

// Mounted once at the console root; renders requests made by confirmAction().
export function ConfirmHost() {
  const request = useSyncExternalStore(subscribeConfirm, pendingConfirm);
  if (!request) return null;
  return (
    <ConfirmDialog key={request.id} {...request} onResult={request.resolve} />
  );
}
