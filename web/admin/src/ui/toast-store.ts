import type { ReactNode } from "react";

export type ToastTone = "info" | "success" | "danger";
export type ToastOptions = {
  tone?: ToastTone;
  // One action, e.g. "Undo" or "Open release".
  action?: { label: string; onClick: () => void };
  // Milliseconds; 0 keeps it until dismissed. Danger toasts never auto-hide.
  duration?: number;
};
export type Toast = ToastOptions & { id: number; message: ReactNode };

let toasts: Toast[] = [],
  nextId = 1;
const listeners = new Set<() => void>();
function publish(next: Toast[]) {
  toasts = next;
  listeners.forEach((listener) => listener());
}

// Shows a short confirmation of something that already happened ("Saved as
// draft"). Errors that need a decision belong in an Alert or a dialog.
// Returns the toast id for dismissToast.
export function toast(message: ReactNode, options: ToastOptions = {}) {
  const id = nextId++;
  publish([...toasts.slice(-3), { ...options, id, message }]);
  return id;
}
export function dismissToast(id: number) {
  publish(toasts.filter((t) => t.id !== id));
}
export const subscribeToasts = (listener: () => void) => {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
};
export const currentToasts = () => toasts;
