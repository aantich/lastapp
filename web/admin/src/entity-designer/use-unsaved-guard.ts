import { useEffect } from "react";
import { beforeNavigateEvent, type BeforeNavigateEvent } from "../console-url";
import { discardChanges } from "./shared";

// While the draft has unsaved edits, leaving the page asks first: the
// browser's unload prompt, and a confirmation before console navigation.
export function useUnsavedGuard(dirty: boolean) {
  useEffect(() => {
    if (!dirty) return;
    const fn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    const navigate = (e: Event) => {
      e.preventDefault();
      void discardChanges().then((discard) => {
        if (discard) (e as BeforeNavigateEvent).detail.proceed();
      });
    };
    window.addEventListener("beforeunload", fn);
    window.addEventListener(beforeNavigateEvent, navigate);
    return () => {
      window.removeEventListener("beforeunload", fn);
      window.removeEventListener(beforeNavigateEvent, navigate);
    };
  }, [dirty]);
}
