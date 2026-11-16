import type { ReactNode } from "react";

export type ConfirmOptions = {
  title: string;
  message?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  // Destructive actions use the danger button style.
  danger?: boolean;
};
export type ConfirmRequest = ConfirmOptions & {
  id: number;
  resolve: (confirmed: boolean) => void;
};

let pending: ConfirmRequest | null = null,
  nextId = 1;
const listeners = new Set<() => void>();

function publish(request: ConfirmRequest | null) {
  pending = request;
  listeners.forEach((listener) => listener());
}

// Asks the user through the console's ConfirmHost instead of window.confirm().
// Resolves true on confirm and false on cancel/Escape. Opening a second
// confirmation cancels the first.
export function confirmAction(options: ConfirmOptions): Promise<boolean> {
  pending?.resolve(false);
  return new Promise((resolve) => {
    const request: ConfirmRequest = {
      ...options,
      id: nextId++,
      resolve: (confirmed) => {
        if (pending !== request) return;
        publish(null);
        resolve(confirmed);
      },
    };
    publish(request);
  });
}

export function subscribeConfirm(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export const pendingConfirm = () => pending;
