import { useEffect, useLayoutEffect, useRef, useState } from "react";

// Everything a keyboard user can reach inside a dialog. Custom Select
// triggers are buttons; JSON fields are textareas.
const focusableSelector = [
  "button:not(:disabled)",
  "a[href]",
  "input:not(:disabled)",
  "select:not(:disabled)",
  "textarea:not(:disabled)",
  "summary",
  '[contenteditable]:not([contenteditable="false"])',
  '[tabindex]:not([tabindex="-1"])',
].join(", ");

// Open dialogs, innermost last. Only the innermost one answers Escape/Tab, so a
// confirmation opened above a form closes alone.
const stack: object[] = [];

// Shared by every modal surface: the appearance drawer, navigation search,
// form dialogs and confirmations. Focuses the dialog, traps Tab inside it,
// closes on Escape and returns focus to the element that opened it.
export function useDialogFocus(close: () => void, active = true) {
  const ref = useRef<HTMLElement>(null);
  // The element focused when the dialog first rendered: a control inside
  // the dialog with autoFocus takes focus before the effect below runs.
  const [opener] = useState(() => document.activeElement as HTMLElement | null);
  const onClose = useRef(close);
  useLayoutEffect(() => {
    onClose.current = close;
  });
  useEffect(() => {
    if (!active) return;
    const token = {};
    stack.push(token);
    const focused = document.activeElement as HTMLElement | null;
    const previous =
      focused && ref.current?.contains(focused) ? opener : focused;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const focusable = () =>
      Array.from(
        ref.current?.querySelectorAll<HTMLElement>(focusableSelector) || [],
      ).filter((el) => el.getClientRects().length);
    (
      ref.current?.querySelector<HTMLElement>("[data-autofocus]") ||
      focusable()[0] ||
      ref.current
    )?.focus();
    const handleKey = (event: KeyboardEvent) => {
      if (stack.at(-1) !== token) return;
      if (event.key === "Escape") {
        event.preventDefault();
        onClose.current();
      }
      if (event.key !== "Tab") return;
      const elements = focusable();
      const first = elements[0],
        last = elements.at(-1);
      if (!first) {
        event.preventDefault();
        return;
      }
      if (
        event.shiftKey &&
        (document.activeElement === first ||
          !ref.current?.contains(document.activeElement))
      ) {
        event.preventDefault();
        last?.focus();
      } else if (
        !event.shiftKey &&
        (document.activeElement === last ||
          !ref.current?.contains(document.activeElement))
      ) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("keydown", handleKey);
      stack.splice(stack.indexOf(token), 1);
      document.body.style.overflow = overflow;
      if (previous?.isConnected) previous.focus();
    };
  }, [active, opener]);
  return ref;
}
