import { useSyncExternalStore } from "react";
import { Button } from "./button";
import { Dialog } from "./dialog";
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
  return (
    <Dialog
      role="alertdialog"
      size="sm"
      className="confirm-modal"
      title={title}
      description={message}
      titleId="confirm-title"
      describedBy="confirm-message"
      onClose={() => onResult(false)}
      footer={
        <>
          <Button
            data-autofocus={danger || undefined}
            onClick={() => onResult(false)}
          >
            {cancelLabel}
          </Button>
          <Button
            variant={danger ? "danger" : "primary"}
            data-autofocus={!danger || undefined}
            onClick={() => onResult(true)}
          >
            {confirmLabel}
          </Button>
        </>
      }
    />
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
